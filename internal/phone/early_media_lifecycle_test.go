package phone

import (
	"net"
	"testing"
	"time"

	"github.com/iniwex5/vowifi-go/runtimehost/voicehost"
)

func TestEarlyMediaUpdateAttachesOutboundWithoutAnswering(t *testing.T) {
	for _, status := range []string{StatusCalling, StatusRinging} {
		t.Run(status, func(t *testing.T) {
			gateway, store := newFakeVoiceGateway(), newMemoryCallStore()
			service := newPhoneTestService(t, gateway, store, time.Second)
			addStubMedia(t, service, "early-media", "admin", "early-lease")
			view, err := service.StartCall(StartCallRequest{
				Owner: "admin", DeviceID: "dev-1", Callee: "+1555550100", MediaID: "early-media", Lease: "early-lease",
			})
			if err != nil {
				t.Fatal(err)
			}
			if status == StatusRinging {
				gateway.emitEvent(voicehost.CallEvent{Type: "CallRinging", DeviceID: "dev-1", CallID: view.CallID})
			}
			relay := newEarlyMediaTestRelay(t)
			gateway.activeSnapshots["dev-1"] = voicehost.CallSnapshot{
				CallID: view.CallID, DeviceID: "dev-1", ClientSDP: g711EndpointSDP(relay.LocalAddr().(*net.UDPAddr).Port, "PCMU", 0),
			}
			gateway.emitEvent(voicehost.CallEvent{Type: "CallMediaUpdated", DeviceID: "dev-1", CallID: view.CallID, State: "ringing"})
			media := service.media.Get("early-media")
			endpoint, _, attached := media.endpoint()
			if !attached || endpoint.Address.Port != relay.LocalAddr().(*net.UDPAddr).Port {
				t.Fatalf("early endpoint = %+v, attached=%t", endpoint, attached)
			}
			// An RTP priming packet proves this is a usable bridge, not just a state flag.
			readUDPRTP(t, relay)
			assertEarlyMediaCallNotAnswered(t, service, store, view.CallID, status)
		})
	}
}

func TestEarlyMediaAttachmentFailureDoesNotHangup(t *testing.T) {
	for _, remoteSDP := range []string{"", "invalid SDP"} {
		t.Run(remoteSDP, func(t *testing.T) {
			gateway, store := newFakeVoiceGateway(), newMemoryCallStore()
			service := newPhoneTestService(t, gateway, store, time.Second)
			addStubMedia(t, service, "early-media", "admin", "early-lease")
			view, err := service.StartCall(StartCallRequest{
				Owner: "admin", DeviceID: "dev-1", Callee: "+1555550100", MediaID: "early-media", Lease: "early-lease",
			})
			if err != nil {
				t.Fatal(err)
			}
			gateway.activeSnapshots["dev-1"] = voicehost.CallSnapshot{CallID: view.CallID, DeviceID: "dev-1", ClientSDP: remoteSDP}
			gateway.emitEvent(voicehost.CallEvent{Type: "CallMediaUpdated", DeviceID: "dev-1", CallID: view.CallID})
			if _, _, attached := service.media.Get("early-media").endpoint(); attached {
				t.Fatal("invalid early SDP attached media")
			}
			select {
			case id := <-gateway.hangupCalls:
				t.Fatalf("early media failure hung up call %s", id)
			default:
			}
			assertEarlyMediaCallNotAnswered(t, service, store, view.CallID, StatusCalling)
		})
	}
}

func TestEarlyMediaUpdateDoesNotAutoAttachIncomingCall(t *testing.T) {
	gateway, store := newFakeVoiceGateway(), newMemoryCallStore()
	service := newPhoneTestService(t, gateway, store, time.Second)
	gateway.emitIncoming(voicehost.IncomingCall{DeviceID: "dev-1", CallID: "incoming-early", Caller: "+1555550100", OfferSDP: testPlainSDP})
	addStubMedia(t, service, "early-media", "admin", "early-lease")
	service.mu.Lock()
	service.calls["incoming-early"].mediaID = "early-media"
	service.mu.Unlock()
	relay := newEarlyMediaTestRelay(t)
	gateway.activeSnapshots["dev-1"] = voicehost.CallSnapshot{
		CallID: "incoming-early", DeviceID: "dev-1", ClientSDP: g711EndpointSDP(relay.LocalAddr().(*net.UDPAddr).Port, "PCMU", 0),
	}
	gateway.emitEvent(voicehost.CallEvent{Type: "CallMediaUpdated", DeviceID: "dev-1", CallID: "incoming-early", State: "ringing"})
	if _, _, attached := service.media.Get("early-media").endpoint(); attached {
		t.Fatal("incoming call audio attached before user answered")
	}
	assertEarlyMediaCallNotAnswered(t, service, store, "incoming-early", StatusRinging)
}

func TestEarlyMediaEventBeforeStartReturnsIsReplayed(t *testing.T) {
	gateway, store := newFakeVoiceGateway(), newMemoryCallStore()
	service := newPhoneTestService(t, gateway, store, time.Second)
	addStubMedia(t, service, "early-media", "admin", "early-lease")
	relay := newEarlyMediaTestRelay(t)
	gateway.activeSnapshots["dev-1"] = voicehost.CallSnapshot{
		CallID: "outbound-1", DeviceID: "dev-1", ClientSDP: g711EndpointSDP(relay.LocalAddr().(*net.UDPAddr).Port, "PCMU", 0),
	}
	gateway.beginEvents = []voicehost.CallEvent{{Type: "CallMediaUpdated", DeviceID: "dev-1", CallID: "outbound-1", State: "ringing"}}
	view, err := service.StartCall(StartCallRequest{
		Owner: "admin", DeviceID: "dev-1", Callee: "+1555550100", MediaID: "early-media", Lease: "early-lease",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, attached := service.media.Get("early-media").endpoint(); !attached {
		t.Fatal("early SDP event received during BeginCall was lost")
	}
	readUDPRTP(t, relay)
	assertEarlyMediaCallNotAnswered(t, service, store, view.CallID, StatusCalling)
}

func newEarlyMediaTestRelay(t *testing.T) *net.UDPConn {
	t.Helper()
	relay, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = relay.Close() })
	return relay
}

func assertEarlyMediaCallNotAnswered(t *testing.T, service *Service, store *memoryCallStore, callID, status string) {
	t.Helper()
	service.mu.RLock()
	call := service.calls[callID]
	gotStatus, answered, terminal := call.view.Status, call.view.AnsweredAt, call.terminal
	service.mu.RUnlock()
	if gotStatus != status || answered != nil || terminal {
		t.Fatalf("early media changed call lifecycle: status=%q answered=%v terminal=%t", gotStatus, answered, terminal)
	}
	record := store.record(callID)
	if record.Status != status || record.AnsweredAt != nil {
		t.Fatalf("early media recorded a false answer: %+v", record)
	}
	service.events.mu.Lock()
	defer service.events.mu.Unlock()
	for _, event := range service.events.events {
		if event.Type == "call_answered" && event.Call.CallID == callID {
			t.Fatal("early media published call_answered")
		}
	}
}
