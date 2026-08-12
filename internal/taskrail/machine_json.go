package taskrail

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"
)

// Strict JSON reading shared by the machine decoders. Every helper rejects
// instead of coercing: the machine contract fixes each member's kind,
// nullability, and enum, so a lenient reading would hand an agent a document the
// producer never promised.

// errMalformedJSON marks input the token walk cannot interpret. The structural
// decode reports such a document precisely, so the walk only has to stop.
var errMalformedJSON = errors.New("malformed JSON")

// checkDocumentFraming enforces the one-object, one-document encoding rule:
// UTF-8 without BOM, no duplicate keys anywhere, and no trailing value.
func checkDocumentFraming(data []byte) error {
	if bytes.HasPrefix(data, []byte("\xef\xbb\xbf")) {
		return errors.New("document begins with a UTF-8 byte order mark")
	}
	if !utf8.Valid(data) {
		return errors.New("document is not valid UTF-8")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	err := rejectDuplicateKeys(dec, "")
	if errors.Is(err, errMalformedJSON) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("document has a trailing value")
	}
	return nil
}

// rejectDuplicateKeys walks one JSON value, reporting a repeated member with the
// dotted path a reader can locate in the document.
func rejectDuplicateKeys(dec *json.Decoder, path string) error {
	token, err := dec.Token()
	if err != nil {
		return errMalformedJSON
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]struct{}{}
		for dec.More() {
			keyToken, err := dec.Token()
			if err != nil {
				return errMalformedJSON
			}
			key, _ := keyToken.(string)
			member := key
			if path != "" {
				member = path + "." + key
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("document repeats member %q", member)
			}
			seen[key] = struct{}{}
			if err := rejectDuplicateKeys(dec, member); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := rejectDuplicateKeys(dec, path); err != nil {
				return err
			}
		}
	}
	if _, err := dec.Token(); err != nil { // consume the closing delimiter
		return errMalformedJSON
	}
	return nil
}

// isCanonicalCommandPath applies the companion's command grammar plus the one
// detectable form of its "contains no executable name" rule: a leading
// `taskrail` token, which no command path may start with.
func isCanonicalCommandPath(command string) bool {
	if !canonicalCommandPath.MatchString(command) {
		return false
	}
	return command != "taskrail" && !strings.HasPrefix(command, "taskrail ")
}

// decodeJSONInteger accepts only a JSON integer token. UseNumber keeps the
// literal text, because unmarshalling straight into json.Number would also
// accept a quoted "1", and Int64 alone would accept "1.0" or "-0".
func decodeJSONInteger(raw json.RawMessage) (int64, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return 0, false
	}
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	integer, err := number.Int64()
	if err != nil || number.String() != fmt.Sprint(integer) {
		return 0, false
	}
	return integer, true
}

func isJSONNull(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
}

func strictObject(raw json.RawMessage, what string) (map[string]json.RawMessage, error) {
	if isJSONNull(raw) {
		return nil, fmt.Errorf("%s is null", what)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("%s is not a JSON object", what)
	}
	return obj, nil
}

// exactMembers accepts an object only when its member set equals names, so both
// unknown and missing members are rejected.
func exactMembers(obj map[string]json.RawMessage, what string, names []string) error {
	for _, name := range names {
		if _, ok := obj[name]; !ok {
			return fmt.Errorf("%s is missing member %q", what, name)
		}
	}
	var unknown []string
	for name := range obj {
		if !slices.Contains(names, name) {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		slices.Sort(unknown)
		return fmt.Errorf("%s has unknown member %q", what, unknown[0])
	}
	return nil
}

func stringMember(obj map[string]json.RawMessage, what, name string) (string, error) {
	raw, ok := obj[name]
	if !ok {
		return "", fmt.Errorf("%s is missing member %q", what, name)
	}
	if isJSONNull(raw) {
		return "", fmt.Errorf("%s member %q is null", what, name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s member %q is not a string", what, name)
	}
	if value == "" {
		return "", fmt.Errorf("%s member %q is empty", what, name)
	}
	return value, nil
}

func nullableStringMember(obj map[string]json.RawMessage, what, name string) (*string, error) {
	if isJSONNull(obj[name]) {
		return nil, nil
	}
	value, err := stringMember(obj, what, name)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func enumMember(obj map[string]json.RawMessage, what, name string, allowed []string) (string, error) {
	value, err := stringMember(obj, what, name)
	if err != nil {
		return "", err
	}
	if !slices.Contains(allowed, value) {
		return "", fmt.Errorf("%s member %q is not an allowed value", what, name)
	}
	return value, nil
}

func commandMember(obj map[string]json.RawMessage, what, name string) (string, error) {
	value, err := stringMember(obj, what, name)
	if err != nil {
		return "", err
	}
	if !isCanonicalCommandPath(value) {
		return "", fmt.Errorf("%s member %q is not a canonical command path", what, name)
	}
	return value, nil
}

func fixedMember(obj map[string]json.RawMessage, what, name, want string) (string, error) {
	value, err := stringMember(obj, what, name)
	if err != nil {
		return "", err
	}
	if value != want {
		return "", fmt.Errorf("%s member %q must be %q", what, name, want)
	}
	return value, nil
}

func boolMember(obj map[string]json.RawMessage, what, name string) (bool, error) {
	raw, ok := obj[name]
	if !ok {
		return false, fmt.Errorf("%s is missing member %q", what, name)
	}
	// Unmarshalling JSON null into a bool is a silent no-op, which would report
	// a malformed required member as an honest false.
	if isJSONNull(raw) {
		return false, fmt.Errorf("%s member %q is null", what, name)
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, fmt.Errorf("%s member %q is not a boolean", what, name)
	}
	return value, nil
}

func arrayMember(raw json.RawMessage, what, name string) ([]json.RawMessage, error) {
	if isJSONNull(raw) {
		return nil, fmt.Errorf("%s member %q is null", what, name)
	}
	var elements []json.RawMessage
	if err := json.Unmarshal(raw, &elements); err != nil {
		return nil, fmt.Errorf("%s member %q is not an array", what, name)
	}
	return elements, nil
}
