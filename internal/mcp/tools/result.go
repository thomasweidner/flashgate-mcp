package tools

import (
	"encoding/json"
	"errors"
	"mime"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

type PayloadKind string

const (
	MetadataPayload   PayloadKind = "metadata"
	StructuredPayload PayloadKind = "structured"
	TextPayload       PayloadKind = "text"
	BinaryPayload     PayloadKind = "binary/media"
)

type TransferMode string

const (
	InlineTransfer   TransferMode = "inline"
	PagedTransfer    TransferMode = "paged"
	ResourceTransfer TransferMode = "resource"
)

type ResultState string

const (
	CompleteResult ResultState = "complete"
	PartialResult  ResultState = "partial"
	PendingResult  ResultState = "pending"
)

const (
	defaultVariant      = "complete"
	metadataParityBytes = 8 * 1024
	maxMetadataBytes    = 256 * 1024
	maxStructuredBytes  = 16 * 1024 * 1024
)

// ResultVariant describes a structured object and its ordered content parts.
// The active adapter supports complete inline results. Other modes and states
// require a delivery mechanism before a tool can register them.
type ResultVariant struct {
	Name            string
	ObjectKind      PayloadKind
	Parts           []PayloadKind
	Mode            TransferMode
	State           ResultState
	MaxObjectBytes  int
	MaxPayloadBytes int64
}

// ResultContract declares the successful variants of one tool.
type ResultContract []ResultVariant

func metadataContract(names ...string) ResultContract {
	contract := make(ResultContract, 0, len(names))
	for _, name := range names {
		contract = append(contract, ResultVariant{
			Name: name, ObjectKind: MetadataPayload, Mode: InlineTransfer,
			State: CompleteResult, MaxObjectBytes: maxMetadataBytes,
		})
	}
	return contract
}

// Check path echoes before a mutation. The allowance covers the fixed fields
// and punctuation of the current filesystem receipts.
func validateReceiptPaths(paths ...string) error {
	if !resultStringsFit(maxMetadataBytes-1024, paths...) {
		return mapFilesystemError(fs.ErrLimitExceeded)
	}
	return nil
}

func (contract ResultContract) validatedCopy() (ResultContract, error) {
	if len(contract) == 0 {
		return nil, errors.New("tool must declare a result variant")
	}
	seen := make(map[string]bool, len(contract))
	result := slices.Clone(contract)
	for index, variant := range result {
		if strings.TrimSpace(variant.Name) == "" || seen[variant.Name] ||
			variant.Mode != InlineTransfer || variant.State != CompleteResult {
			return nil, errors.New("invalid or unsupported result variant")
		}
		seen[variant.Name] = true
		objectLimit := maxMetadataBytes
		switch variant.ObjectKind {
		case MetadataPayload:
		case StructuredPayload:
			objectLimit = maxStructuredBytes
		default:
			return nil, errors.New("result object must be metadata or structured data")
		}
		if variant.MaxObjectBytes <= 0 || variant.MaxObjectBytes > objectLimit ||
			variant.MaxPayloadBytes < 0 || (len(variant.Parts) == 0) != (variant.MaxPayloadBytes == 0) {
			return nil, errors.New("invalid result limits")
		}
		if len(variant.Parts) > 0 && variant.ObjectKind != MetadataPayload {
			return nil, errors.New("content parts require derived metadata")
		}
		for _, kind := range variant.Parts {
			if kind != TextPayload && kind != BinaryPayload {
				return nil, errors.New("unsupported result part kind")
			}
		}
		result[index].Parts = slices.Clone(variant.Parts)
	}
	return result, nil
}

// ResultPart is a single payload in an inline result.
type ResultPart struct {
	kind PayloadKind
	text string
	mime string
	data []byte
}

func TextPart(text string) (ResultPart, error) {
	if !utf8.ValidString(text) {
		return ResultPart{}, errInvalidToolResult
	}
	return ResultPart{kind: TextPayload, text: text}, nil
}

// MediaPart represents inline MCP image or audio content. The domain supplies
// the media type. File type detection and arbitrary binary delivery are separate.
func MediaPart(mediaType string, data []byte) ResultPart {
	return ResultPart{kind: BinaryPayload, mime: mediaType, data: data}
}

// ToolResult holds either a standalone object or content parts. Only the
// adapter constructs the wire envelope and the metadata for content parts.
type ToolResult struct {
	variant string
	object  resultObject
	parts   []ResultPart
}

func NewToolResult(variant string, object resultObject) ToolResult {
	return ToolResult{variant: variant, object: object}
}

func NewPayloadResult(variant string, parts ...ResultPart) ToolResult {
	return ToolResult{variant: variant, parts: parts}
}

func (result ToolResult) Variant() string { return result.variant }

var errInvalidToolResult = errors.New("invalid tool result")
var errToolResultLimit = errors.New("tool result limit exceeded")

func (contract ResultContract) wrap(result ToolResult) (protocol.CallToolResult, error) {
	for _, variant := range contract {
		if variant.Name == result.variant {
			return variant.wrap(result)
		}
	}
	return protocol.CallToolResult{}, errInvalidToolResult
}

func (variant ResultVariant) wrap(result ToolResult) (protocol.CallToolResult, error) {
	if len(variant.Parts) != len(result.parts) {
		return protocol.CallToolResult{}, errInvalidToolResult
	}
	if len(result.parts) == 0 {
		object, err := encodeResultObject(result.object, variant.ObjectKind, variant.MaxObjectBytes)
		if err != nil {
			return protocol.CallToolResult{}, err
		}
		text := "See structuredContent for the result."
		if variant.ObjectKind == MetadataPayload && len(object) <= metadataParityBytes {
			text = string(object)
		}
		return protocol.CallToolResult{
			Content: []protocol.ContentBlock{protocol.NewTextContent(text)}, StructuredContent: object,
		}, nil
	}
	if result.object != nil || variant.ObjectKind != MetadataPayload {
		return protocol.CallToolResult{}, errInvalidToolResult
	}
	return variant.wrapParts(result.parts)
}

type payloadMetadata struct {
	Size int64 `json:"size"`
}

func (variant ResultVariant) wrapParts(parts []ResultPart) (protocol.CallToolResult, error) {
	blocks := make([]protocol.ContentBlock, 0, len(parts))
	remaining := variant.MaxPayloadBytes
	for index, part := range parts {
		if part.kind != variant.Parts[index] {
			return protocol.CallToolResult{}, errInvalidToolResult
		}
		block, rawBytes, err := part.block(remaining)
		if err != nil {
			return protocol.CallToolResult{}, err
		}
		remaining -= rawBytes
		blocks = append(blocks, block)
	}
	size := variant.MaxPayloadBytes - remaining
	if len(`{"size":}`)+decimalSize(size) > variant.MaxObjectBytes {
		return protocol.CallToolResult{}, errToolResultLimit
	}
	object, err := json.Marshal(payloadMetadata{Size: size})
	if err != nil {
		return protocol.CallToolResult{}, errInvalidToolResult
	}
	return protocol.CallToolResult{Content: blocks, StructuredContent: object}, nil
}

// block validates the fields for the part kind and reports the raw size of the
// payload it emits, before wire escaping or base64 encoding.
func (part ResultPart) block(maxRawBytes int64) (protocol.ContentBlock, int64, error) {
	if part.kind == TextPayload {
		if part.mime != "" || part.data != nil {
			return nil, 0, errInvalidToolResult
		}
		rawBytes := int64(len(part.text))
		if rawBytes > maxRawBytes {
			return nil, 0, errToolResultLimit
		}
		if !utf8.ValidString(part.text) {
			return nil, 0, errInvalidToolResult
		}
		return protocol.NewTextContent(part.text), rawBytes, nil
	}
	if part.kind != BinaryPayload || part.text != "" || len(part.data) == 0 {
		return nil, 0, errInvalidToolResult
	}
	rawBytes := int64(len(part.data))
	if rawBytes > maxRawBytes {
		return nil, 0, errToolResultLimit
	}
	mediaType, params, err := mime.ParseMediaType(part.mime)
	blockType, subtype, _ := strings.Cut(mediaType, "/")
	if err != nil || len(params) != 0 || subtype == "" ||
		strings.Contains(subtype, "*") || (blockType != "image" && blockType != "audio") {
		return nil, 0, errInvalidToolResult
	}
	return protocol.MediaContent{Type: blockType, MIMEType: mediaType, Data: part.data}, rawBytes, nil
}
