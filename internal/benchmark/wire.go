package benchmark

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/bits"
	"reflect"
	"strconv"
	"unicode/utf8"
)

// WireSource distinguishes actual product routes, candidate-policy routes and
// test-only deliveries. An unavailable route has no measurement.
type WireSource string

const (
	WireProduction  WireSource = "production"
	WireCandidate   WireSource = "candidate"
	WireSynthetic   WireSource = "synthetic"
	WireUnavailable WireSource = "unavailable"
)

// WireIdentity binds one logical retrieval. For a non-tool producer, Route
// names the actual entry point and Tool and Variant are empty.
type WireIdentity struct {
	Source                                  WireSource
	Revision, Profile, Route, Tool, Variant string
	Fixture, Transport, Transfer, State     string
	Outcome                                 string
	Contract                                string
}

// WireClass refines the existing result kinds only for measurement. A frame
// can carry several classes and may carry none of the reference payload.
type WireClass string

const (
	WireMetadata   WireClass = "metadata"
	WireObject     WireClass = "structured-object"
	WireCollection WireClass = "structured-collection"
	WireText       WireClass = "text"
	WireBinary     WireClass = "binary"
	WireImage      WireClass = "image"
	WireAudio      WireClass = "audio"
)

type WirePayload struct {
	Class    WireClass
	Bytes    []byte
	Encoding WireEncoding
}

// WireEncoding selects accounting independently of the semantic payload class.
type WireEncoding string

const (
	WireUTF8   WireEncoding = "utf8"
	WireJSON   WireEncoding = "json"
	WireBase64 WireEncoding = "base64"
)

type WireRole string

const (
	WireUseful        WireRole = "useful"
	WireDomain        WireRole = "domain-metadata"
	WireCompatibility WireRole = "compatibility"
	WireMeta          WireRole = "revision-meta"
	WireResultType    WireRole = "revision-result-type"
	WireCache         WireRole = "revision-cache"
	WireCursor        WireRole = "cursor"
)

// WireField locates an exact JSON value, using property names or array
// indices. Useful fields consume reference bytes in order. Continue retains
// the current reference part for the next range or chunk.
type WireField struct {
	Path     []string
	Role     WireRole
	Continue bool
}

// WireFrame binds producer-owned framing and the independently specified MCP
// body. Body selects result, error or params. For a response ID, use its
// exact JSON representation. A notification has an empty ID and a Method.
type WireFrame struct {
	Wire                       []byte
	Prefix, Suffix             []byte
	ID, Method, Body, Delivery string
	State                      string
	Expected                   json.RawMessage
	Fields                     []WireField
	EncodingPasses             uint64
	DecodingPasses             uint64
	PayloadCopies              uint64
	ExplicitCopyBytes          uint64
	CopiesObserved             bool
}

type WireFixture struct {
	Identity  WireIdentity
	Reference []WirePayload
	Frames    []WireFrame
}

// WireCounts are additive across frames except tokens, which are calculated
// from the total UTF-8 response bytes for a logical retrieval.
type WireCounts struct {
	ResponseBytes, ResultBytes, FramingBytes, EnvelopeBytes uint64
	UsefulBytes, RawPayloadBytes, EncodedPayloadBytes       uint64
	EscapingBytes, Base64Bytes, PayloadFormattingBytes      uint64
	DomainBytes, CompatibilityBytes, MetadataBytes          uint64
	RevisionMetaBytes, ResultTypeBytes, CacheBytes          uint64
	CursorBytes                                             uint64
	EncodingPasses, DecodingPasses                          uint64
	PayloadCopies, ExplicitCopyBytes                        uint64
}

type WirePartMeasurement struct {
	Delivery, State    string
	Counts             WireCounts
	ApproxTokensBytes4 uint64
	CopiesObserved     bool
}

type WireMeasurement struct {
	SchemaVersion                string
	Identity                     WireIdentity
	PayloadClasses               []WireClass
	Frames                       []WirePartMeasurement
	Counts                       WireCounts
	CopiesObserved               bool
	ApproxTokensBytes4           uint64
	WireAmplification            *float64
	ApproxTokenCostPerUsefulByte *float64
}

const WireMeasurementVersion = "flashgate-benchmark-wire/v1"

// MeasureWire counts observed frames after verifying exact semantic body and
// payload placement. JSON syntax, punctuation and string quotes are charged
// to the envelope or residual metadata. Field attribution counts value bytes.
func MeasureWire(fixture WireFixture) (WireMeasurement, error) {
	id := fixture.Identity
	if (id.Source != WireProduction && id.Source != WireCandidate && id.Source != WireSynthetic) ||
		id.Revision == "" || id.Profile == "" || id.Route == "" || id.Fixture == "" ||
		id.Transport == "" || id.Transfer == "" || id.State == "" || id.Outcome == "" ||
		(id.Tool == "") != (id.Variant == "") || len(fixture.Frames) == 0 {
		return WireMeasurement{}, fmt.Errorf("incomplete or unavailable wire fixture")
	}
	if id.Outcome == "error" && len(fixture.Reference) != 0 {
		return WireMeasurement{}, fmt.Errorf("error fixture cannot deliver useful payload")
	}
	reference := wireReference{parts: fixture.Reference}
	measurement := WireMeasurement{SchemaVersion: WireMeasurementVersion, Identity: id, CopiesObserved: true}
	for _, payload := range fixture.Reference {
		if payload.Class == "" {
			return WireMeasurement{}, fmt.Errorf("missing wire payload class")
		}
		measurement.PayloadClasses = append(measurement.PayloadClasses, payload.Class)
	}
	if len(fixture.Reference) == 0 {
		measurement.PayloadClasses = []WireClass{WireMetadata}
	}
	for index, frame := range fixture.Frames {
		counts, err := measureWireFrame(frame, &reference)
		if err != nil {
			return WireMeasurement{}, fmt.Errorf("frame %d: %w", index+1, err)
		}
		if err := addWireCounts(&measurement.Counts, counts); err != nil {
			return WireMeasurement{}, err
		}
		state := frame.State
		if state == "" {
			state = id.State
		}
		measurement.Frames = append(measurement.Frames, WirePartMeasurement{frame.Delivery, state, counts,
			approxTokensBytes4(counts.ResponseBytes), frame.CopiesObserved})
		measurement.CopiesObserved = measurement.CopiesObserved && frame.CopiesObserved
	}
	if len(reference.parts) != 0 {
		return WireMeasurement{}, fmt.Errorf("reference payload was not fully delivered")
	}
	total := measurement.Counts
	measurement.ApproxTokensBytes4 = approxTokensBytes4(total.ResponseBytes)
	if total.UsefulBytes != 0 {
		amplification := float64(total.ResponseBytes) / float64(total.UsefulBytes)
		tokenCost := float64(measurement.ApproxTokensBytes4) / float64(total.UsefulBytes)
		measurement.WireAmplification = &amplification
		measurement.ApproxTokenCostPerUsefulByte = &tokenCost
	}
	return measurement, nil
}

func measureWireFrame(frame WireFrame, reference *wireReference) (WireCounts, error) {
	if frame.Delivery == "" || len(frame.Prefix) >= len(frame.Wire) ||
		len(frame.Suffix) >= len(frame.Wire)-len(frame.Prefix) ||
		!bytes.HasPrefix(frame.Wire, frame.Prefix) || !bytes.HasSuffix(frame.Wire, frame.Suffix) {
		return WireCounts{}, fmt.Errorf("invalid delivery or framing")
	}
	if !frame.CopiesObserved && (frame.EncodingPasses|frame.DecodingPasses|frame.PayloadCopies|frame.ExplicitCopyBytes) != 0 {
		return WireCounts{}, fmt.Errorf("unobserved copy or serialization counts")
	}
	jsonBytes := frame.Wire[len(frame.Prefix) : len(frame.Wire)-len(frame.Suffix)]
	if !utf8.Valid(jsonBytes) {
		return WireCounts{}, fmt.Errorf("invalid wire JSON UTF-8")
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(jsonBytes, &envelope); err != nil {
		return WireCounts{}, err
	}
	body := envelope[frame.Body]
	if !validWireEnvelope(frame, envelope) || len(body) == 0 {
		return WireCounts{}, fmt.Errorf("unexpected wire envelope")
	}
	// Decode with the existing duplicate-key rejection. The envelope must
	// also reject repeated IDs and body keys before field extraction.
	actual, err := decodeWireJSON(jsonBytes)
	if err != nil {
		return WireCounts{}, fmt.Errorf("ambiguous wire JSON: %v", err)
	}
	expected, err := decodeWireJSON(frame.Expected)
	if err != nil {
		return WireCounts{}, fmt.Errorf("invalid semantic fixture: %w", err)
	}
	if !reflect.DeepEqual(actual.(map[string]any)[frame.Body], expected) {
		return WireCounts{}, fmt.Errorf("body differs from semantic fixture or payload placement")
	}
	counts := WireCounts{
		ResponseBytes: uint64(len(frame.Wire)), FramingBytes: uint64(len(frame.Prefix) + len(frame.Suffix)),
		EnvelopeBytes: uint64(len(jsonBytes) - len(body)), EncodingPasses: frame.EncodingPasses,
		DecodingPasses: frame.DecodingPasses, PayloadCopies: frame.PayloadCopies, ExplicitCopyBytes: frame.ExplicitCopyBytes,
	}
	if frame.Body == "result" {
		counts.ResultBytes = uint64(len(body))
	}
	if err := measureWireFields(body, frame.Fields, reference, &counts); err != nil {
		return WireCounts{}, err
	}
	return counts, nil
}

func validWireEnvelope(frame WireFrame, envelope map[string]json.RawMessage) bool {
	if len(envelope) != 3 || string(envelope["jsonrpc"]) != `"2.0"` {
		return false
	}
	if frame.Method != "" {
		var method string
		return frame.ID == "" && frame.Body == "params" &&
			json.Unmarshal(envelope["method"], &method) == nil && method == frame.Method
	}
	return frame.ID != "" && string(envelope["id"]) == frame.ID && (frame.Body == "result" || frame.Body == "error")
}

func decodeWireJSON(raw []byte) (any, error) {
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return nil, fmt.Errorf("invalid JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	return decodeUniqueJSONValue(decoder, "$")
}

func measureWireFields(body json.RawMessage, fields []WireField, reference *wireReference, counts *WireCounts) error {
	var prior [][]string
	residual := uint64(len(body))
	for _, field := range fields {
		for _, path := range prior {
			if wirePathsOverlap(path, field.Path) {
				return fmt.Errorf("overlapping wire fields")
			}
		}
		prior = append(prior, field.Path)
		raw, err := wireValue(body, field.Path)
		if err != nil {
			return err
		}
		size := uint64(len(raw))
		if field.Role == WireUseful {
			size, err = reference.consume(raw, field.Continue, counts)
		} else {
			err = chargeWireMetadata(field, size, counts)
		}
		if err != nil {
			return err
		}
		residual -= size // Selected JSON values are disjoint and within body.
	}
	counts.MetadataBytes = residual + counts.DomainBytes + counts.CursorBytes
	return nil
}

func chargeWireMetadata(field WireField, size uint64, counts *WireCounts) error {
	if field.Continue {
		return fmt.Errorf("only useful fields can continue a payload")
	}
	switch field.Role {
	case WireDomain:
		counts.DomainBytes += size
	case WireCompatibility:
		counts.CompatibilityBytes += size
	case WireMeta:
		counts.RevisionMetaBytes += size
	case WireResultType:
		counts.ResultTypeBytes += size
	case WireCache:
		counts.CacheBytes += size
	case WireCursor:
		counts.CursorBytes += size
	default:
		return fmt.Errorf("unknown wire field role %q", field.Role)
	}
	return nil
}

func wirePathsOverlap(a, b []string) bool {
	for index := 0; index < min(len(a), len(b)); index++ {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

func wireValue(raw json.RawMessage, path []string) (json.RawMessage, error) {
	for _, step := range path {
		switch bytes.TrimSpace(raw)[0] {
		case '{':
			var object map[string]json.RawMessage
			if err := json.Unmarshal(raw, &object); err != nil {
				return nil, err
			}
			raw = object[step]
		case '[':
			var array []json.RawMessage
			index, err := strconv.Atoi(step)
			if err != nil || strconv.Itoa(index) != step || json.Unmarshal(raw, &array) != nil ||
				index < 0 || index >= len(array) {
				return nil, fmt.Errorf("invalid wire array index %q", step)
			}
			raw = array[index]
		default:
			return nil, fmt.Errorf("wire path descends through a scalar")
		}
		if len(raw) == 0 {
			return nil, fmt.Errorf("missing wire value at %q", step)
		}
	}
	return raw, nil
}

type wireReference struct {
	parts  []WirePayload
	offset int
}

func (ref *wireReference) consume(raw json.RawMessage, more bool, counts *WireCounts) (uint64, error) {
	if len(ref.parts) == 0 {
		return 0, fmt.Errorf("unexpected useful payload")
	}
	part := ref.parts[0]
	remaining := part.Bytes[ref.offset:]
	payload, err := decodeWirePayload(raw, part.Encoding)
	if err != nil {
		return 0, err
	}
	size := len(payload.bytes)
	if !bytes.HasPrefix(remaining, payload.bytes) ||
		(more && (size == 0 || size == len(remaining))) ||
		(!more && size != len(remaining)) {
		return 0, fmt.Errorf("payload differs from ordered reference")
	}
	if more {
		ref.offset += size
	} else {
		ref.parts = ref.parts[1:]
		ref.offset = 0
	}
	counts.UsefulBytes += uint64(size)
	counts.RawPayloadBytes += uint64(size)
	counts.EncodedPayloadBytes += payload.encoded
	counts.EscapingBytes += payload.escaping
	counts.Base64Bytes += payload.base64
	counts.PayloadFormattingBytes += payload.formatting
	return payload.encoded, nil
}

type decodedWirePayload struct {
	bytes                                 []byte
	encoded, escaping, base64, formatting uint64
}

func decodeWirePayload(raw json.RawMessage, encoding WireEncoding) (decodedWirePayload, error) {
	if encoding == WireJSON {
		if raw[0] != '{' && raw[0] != '[' {
			return decodedWirePayload{}, fmt.Errorf("structured payload has wrong shape")
		}
		canonical, err := CanonicalWireJSON(raw)
		var compact bytes.Buffer
		if err != nil || json.Compact(&compact, raw) != nil || len(canonical) > compact.Len() {
			return decodedWirePayload{}, fmt.Errorf("invalid structured payload encoding: %v", err)
		}
		return decodedWirePayload{
			bytes: canonical, encoded: uint64(len(raw)),
			escaping: uint64(compact.Len() - len(canonical)), formatting: uint64(len(raw) - compact.Len()),
		}, nil
	}
	var value string
	if raw[0] != '"' || json.Unmarshal(raw, &value) != nil || len(value) > len(raw)-2 {
		return decodedWirePayload{}, fmt.Errorf("invalid encoded payload")
	}
	encoded := uint64(len(raw) - 2)
	payload := decodedWirePayload{encoded: encoded, escaping: encoded - uint64(len(value))}
	switch encoding {
	case WireUTF8:
		payload.bytes = []byte(value)
	case WireBase64:
		data, err := base64.StdEncoding.Strict().DecodeString(value)
		if err != nil || base64.StdEncoding.EncodeToString(data) != value {
			return decodedWirePayload{}, fmt.Errorf("invalid canonical base64 payload")
		}
		payload.bytes, payload.base64 = data, uint64(len(value)-len(data))
	default:
		return decodedWirePayload{}, fmt.Errorf("unsupported wire encoding %q", encoding)
	}
	return payload, nil
}

// CanonicalWireJSON fixes object key order and removes transport-only HTML
// escapes when defining the semantic bytes of a structured payload.
func CanonicalWireJSON(raw json.RawMessage) ([]byte, error) {
	value, err := decodeWireJSON(raw)
	if err != nil {
		return nil, err
	}
	var canonical bytes.Buffer
	encoder := json.NewEncoder(&canonical)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(canonical.Bytes(), []byte{'\n'}), nil
}

func addWireCounts(total *WireCounts, part WireCounts) error {
	// Both operands are the same closed set of uint64 counters. Keep the
	// addition checked even when the source is a synthetic measurement.
	dst := reflect.ValueOf(total).Elem()
	src := reflect.ValueOf(part)
	for i := range dst.NumField() {
		sum, carry := bits.Add64(dst.Field(i).Uint(), src.Field(i).Uint(), 0)
		if carry != 0 {
			return fmt.Errorf("wire count overflow in %s", dst.Type().Field(i).Name)
		}
		dst.Field(i).SetUint(sum)
	}
	return nil
}
