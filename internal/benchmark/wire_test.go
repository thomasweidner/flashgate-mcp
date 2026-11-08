package benchmark

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func wireFixture(body string, reference []WirePayload, fields ...WireField) WireFixture {
	return WireFixture{
		Identity: WireIdentity{Source: WireSynthetic, Revision: "fixture-revision", Profile: "fixture-profile",
			Route: "fixture/route", Fixture: "reference", Transport: "lf", Transfer: "inline", State: "complete", Outcome: "success"},
		Reference: reference,
		Frames: []WireFrame{{
			Wire:   []byte(`{"jsonrpc":"2.0","id":1,"result":` + body + "}\n"),
			Suffix: []byte{'\n'}, ID: "1", Body: "result", Delivery: "response",
			Expected: json.RawMessage(body), Fields: fields,
		}},
	}
}

func TestWirePayloadAccounting(t *testing.T) {
	fixture := wireFixture(`{"text":"é\u003c","base64":"AQI=","object":{"z":"\u003c",
 "a":1},"compat":"é<"}`,
		[]WirePayload{{WireText, []byte("é<"), WireUTF8}, {"fixture-new-class", []byte{1, 2}, WireBase64},
			{WireObject, []byte(`{"a":1,"z":"<"}`), WireJSON}},
		WireField{Path: []string{"text"}, Role: WireUseful}, WireField{Path: []string{"base64"}, Role: WireUseful},
		WireField{Path: []string{"object"}, Role: WireUseful}, WireField{Path: []string{"compat"}, Role: WireCompatibility})
	m, err := MeasureWire(fixture)
	if err != nil {
		t.Fatal(err)
	}
	c := m.Counts
	useful := uint64(3 + 2 + len(`{"a":1,"z":"<"}`))
	if c.UsefulBytes != useful || c.RawPayloadBytes != useful || c.EncodedPayloadBytes != useful+14 ||
		c.EscapingBytes != 10 || c.Base64Bytes != 2 || c.PayloadFormattingBytes != 2 ||
		c.CompatibilityBytes != 5 || c.FramingBytes != 1 || m.PayloadClasses[1] != "fixture-new-class" ||
		c.ResponseBytes != uint64(len(fixture.Frames[0].Wire)) || c.ResultBytes != uint64(len(fixture.Frames[0].Expected)) ||
		c.ResultBytes != c.MetadataBytes+c.EncodedPayloadBytes+c.CompatibilityBytes ||
		c.ResponseBytes != c.ResultBytes+c.FramingBytes+c.EnvelopeBytes || m.CopiesObserved ||
		*m.WireAmplification != float64(c.ResponseBytes)/float64(useful) ||
		*m.ApproxTokenCostPerUsefulByte != float64((c.ResponseBytes+3)/4)/float64(useful) {
		t.Fatalf("unexpected wire metrics: %+v", m)
	}
}

func TestWireZeroPayloadAndRPCError(t *testing.T) {
	fixture := wireFixture(`{"receipt":{"created":true}}`, nil, WireField{Path: []string{"receipt"}, Role: WireDomain})
	m, err := MeasureWire(fixture)
	if err != nil || m.Counts.DomainBytes != 16 || m.Counts.UsefulBytes != 0 ||
		m.WireAmplification != nil || m.ApproxTokenCostPerUsefulByte != nil {
		t.Fatalf("zero payload receipt: %+v, %v", m, err)
	}
	frame := &fixture.Frames[0]
	frame.Wire = []byte("{\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32602}}\n")
	frame.Body, frame.Expected, frame.Fields = "error", json.RawMessage(`{"code":-32602}`), nil
	fixture.Identity.Outcome = "error"
	m, err = MeasureWire(fixture)
	if err != nil || m.Counts.ResultBytes != 0 || m.Counts.MetadataBytes != 15 || m.Counts.ResponseBytes == 0 {
		t.Fatalf("JSON-RPC error metrics: %+v, %v", m, err)
	}
}

func TestWireOrderedRangesAndDeliveryBounds(t *testing.T) {
	first := wireFixture(`{"payload":"abc","cursor":"next"}`, []WirePayload{{WireText, []byte("abcdef"), WireUTF8}},
		WireField{Path: []string{"payload"}, Role: WireUseful, Continue: true},
		WireField{Path: []string{"cursor"}, Role: WireCursor})
	second := wireFixture(`{"payload":"def"}`, nil, WireField{Path: []string{"payload"}, Role: WireUseful})
	first.Frames[0].State = "partial"
	first.Frames = append(first.Frames, second.Frames[0])
	m, err := MeasureWire(first)
	if err != nil || len(m.Frames) != 2 || m.Frames[0].State != "partial" || m.Frames[1].State != "complete" ||
		m.Frames[0].Counts.UsefulBytes != 3 || m.Frames[1].Counts.UsefulBytes != 3 ||
		m.Counts.CursorBytes != 6 || m.Counts.UsefulBytes != 6 || m.Counts.FramingBytes != 2 || m.CopiesObserved {
		t.Fatalf("wrong range aggregate: %+v, %v", m, err)
	}
	budget := WireBudget{MaxResponseBytes: 300, MaxResultBytes: 250, MaxOverheadBytes: 300,
		MaxFrameBytes: 45, MaxAmplification: 50000, MaxTokenCost: 13000}
	if err := CheckWireBudget(m, budget, WirePathBudget{}); err == nil || !strings.Contains(err.Error(), "frame bytes") {
		t.Fatalf("per-frame budget did not reject an oversized frame: %v", err)
	}
	first.Frames[0], first.Frames[1] = first.Frames[1], first.Frames[0]
	if _, err := MeasureWire(first); err == nil || !strings.Contains(err.Error(), "ordered reference") {
		t.Fatalf("accepted reordered delivery: %v", err)
	}
}

func TestWireRejectsInconsistentInputs(t *testing.T) {
	for _, test := range []struct {
		name, want string
		mutate     func(*WireFixture)
	}{
		{"unavailable", "unavailable", func(f *WireFixture) { f.Identity.Source = WireUnavailable }},
		{"noncanonical Base64", "base64", func(f *WireFixture) {
			*f = wireFixture(`{"data":"AR=="}`, f.Reference, f.Frames[0].Fields...)
		}},
		{"valid encoding wrong size", "ordered reference", func(f *WireFixture) {
			*f = wireFixture(`{"data":"AQI="}`, f.Reference, f.Frames[0].Fields...)
		}},
		{"duplicate object key", "duplicate", func(f *WireFixture) {
			*f = wireFixture(`{"data":"AQ==","data":"AQ=="}`, f.Reference, f.Frames[0].Fields...)
		}},
		{"contradictory expected object", "invalid semantic fixture", func(f *WireFixture) {
			f.Frames[0].Expected = json.RawMessage(`{"data":"AR==","data":"AQ=="}`)
		}},
		{"duplicate location", "overlapping", func(f *WireFixture) {
			f.Frames[0].Fields = append(f.Frames[0].Fields, f.Frames[0].Fields[0])
		}},
		{"unobserved copies", "unobserved", func(f *WireFixture) { f.Frames[0].PayloadCopies = 1 }},
		{"truncated framing", "framing", func(f *WireFixture) {
			f.Frames[0].Wire = f.Frames[0].Wire[:len(f.Frames[0].Wire)-1]
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := wireFixture(`{"data":"AQ=="}`, []WirePayload{{WireBinary, []byte{1}, WireBase64}},
				WireField{Path: []string{"data"}, Role: WireUseful})
			test.mutate(&f)
			if _, err := MeasureWire(f); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want %q", err, test.want)
			}
		})
	}
}

func TestWireBudgetBoundaries(t *testing.T) {
	if approxTokensBytes4(math.MaxUint64) != 1<<62 ||
		!wireRatioFits(math.MaxUint64, math.MaxUint64, 1000) ||
		wireRatioFits(math.MaxUint64, math.MaxUint64-1, 1000) ||
		wireRatioFits(101, 100, 1009) || !wireRatioFits(101, 100, 1010) {
		t.Fatal("integer ceilings or overflow handling changed")
	}
	total := WireCounts{ExplicitCopyBytes: math.MaxUint64}
	if err := addWireCounts(&total, WireCounts{ExplicitCopyBytes: 1}); err == nil {
		t.Fatal("aggregate accepted an overflowing copy count")
	}
	fixture := wireFixture(`{"data":"abcd"}`, []WirePayload{{WireText, []byte("abcd"), WireUTF8}},
		WireField{Path: []string{"data"}, Role: WireUseful})
	m, err := MeasureWire(fixture)
	if err != nil {
		t.Fatal(err)
	}
	b := WireBudget{MaxResponseBytes: 100, MaxResultBytes: 100, MaxOverheadBytes: 100, MaxAmplification: 20000, MaxTokenCost: 5000}
	if err := CheckWireBudget(m, b, WirePathBudget{}); err != nil {
		t.Fatal(err)
	}
	m.Counts.UsefulBytes++
	if err := CheckWireBudget(m, b, WirePathBudget{}); err == nil {
		t.Fatal("inconsistent metric passed a hard budget")
	}
}
