package phone

import (
	"strings"
	"testing"
)

func TestWidebandPhoneOffersBothDTMFClockRates(t *testing.T) {
	for name, sdp := range map[string]string{
		"offer":    plainAudioSDP(22000, []string{"AMR-WB", "AMR"}),
		"selected": plainSelectedAudioSDP(22000, rtpEndpoint{PayloadType: 104, Codec: "AMR-WB", ClockRate: 16000}),
	} {
		t.Run(name, func(t *testing.T) {
			for _, attr := range []string{"a=rtpmap:96 telephone-event/16000", "a=fmtp:96 0-15", "a=rtpmap:101 telephone-event/8000"} {
				if !strings.Contains(sdp, attr+"\r\n") {
					t.Fatalf("missing %q", attr)
				}
			}
			for _, line := range strings.Split(sdp, "\r\n") {
				if strings.HasPrefix(line, "m=audio ") && (!strings.Contains(" "+line+" ", " 96 ") || !strings.Contains(" "+line+" ", " 101 ")) {
					t.Fatal("DTMF not listed in media formats")
				}
			}
		})
	}
}

func TestNarrowbandPhoneKeepsDTMF8000(t *testing.T) {
	sdp := plainAudioSDP(22000, nil)
	if strings.Contains(sdp, "telephone-event/16000") || !strings.Contains(sdp, "telephone-event/8000") {
		t.Fatal("unexpected narrowband DTMF offer")
	}
}
