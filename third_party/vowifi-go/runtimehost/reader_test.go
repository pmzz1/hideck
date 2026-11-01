package runtimehost

import (
	"testing"

	enginesim "github.com/iniwex5/vowifi-go/engine/sim"
	"github.com/iniwex5/vowifi-go/runtimehost/identity"
)

type readerAKAProviderStub struct{}

func (*readerAKAProviderStub) CalculateAKA(rand16, autn16 []byte) (enginesim.AKAResult, error) {
	return enginesim.AKAResult{}, nil
}

type splitAuthProvider struct{ usimCalls, isimCalls int }

func (p *splitAuthProvider) CalculateAKA(_, _ []byte) (enginesim.AKAResult, error) {
	p.usimCalls++
	return enginesim.AKAResult{RES: []byte{1}}, nil
}

func (p *splitAuthProvider) CalculateISIMAKA(_, _ []byte) (enginesim.AKAResult, error) {
	p.isimCalls++
	return enginesim.AKAResult{RES: []byte{2}}, nil
}

func TestReaderKeepsEPDGUSIMAndIMSISIMSeparate(t *testing.T) {
	p := &splitAuthProvider{}
	a := NewReaderSIMAdapter(p).runtimeSIMAdapter()
	plan := runtimeAuthPlan(&identity.PreparedSession{AuthPlan: identity.AuthPlan{
		EPDGApp: "usim", IMSApp: "isim_strict",
	}})
	epdg, err := a.EPDGSIMProvider(plan).CalculateAKA(nil, nil)
	if err != nil || len(epdg.RES) != 1 || epdg.RES[0] != 1 {
		t.Fatalf("ePDG did not use USIM: result=%v err=%v", epdg.RES, err)
	}
	ims, err := a.IMSAKAProvider(plan).CalculateAKA(nil, nil)
	if err != nil || len(ims.RES) != 1 || ims.RES[0] != 2 {
		t.Fatalf("IMS did not retain strict ISIM: result=%v err=%v", ims.RES, err)
	}
	if p.usimCalls != 1 || p.isimCalls != 1 {
		t.Fatalf("unexpected application calls: USIM=%d ISIM=%d", p.usimCalls, p.isimCalls)
	}
}

func TestNewReaderSIMAdapterExposesInjectedAKAProvider(t *testing.T) {
	provider := &readerAKAProviderStub{}
	adapter := NewReaderSIMAdapter(provider)
	if adapter == nil || adapter.runtimeSIMAdapter() == nil {
		t.Fatal("runtime SIM adapter is unavailable")
	}
	got := adapter.runtimeSIMAdapter().EPDGSIMProvider(runtimeAuthPlan(nil))
	if got != provider {
		t.Fatalf("EPDGSIMProvider() = %T, want injected provider", got)
	}
}
