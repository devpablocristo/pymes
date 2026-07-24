package ar

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type CUIT string

func ParseCUIT(raw string) (CUIT, error) {
	normalized, err := normalizeTaxID(raw)
	if err != nil {
		return "", err
	}
	if len(normalized) != 11 {
		return "", errors.New("CUIT must contain exactly 11 digits")
	}
	if !validCUITDigits(normalized) {
		return "", errors.New("invalid CUIT check digit")
	}
	return CUIT(normalized), nil
}

func normalizeTaxID(raw string) (string, error) {
	var normalized strings.Builder
	for _, character := range strings.TrimSpace(raw) {
		switch {
		case character >= '0' && character <= '9':
			normalized.WriteRune(character)
		case character == '-' || character == ' ':
		default:
			return "", fmt.Errorf("tax identifier contains invalid character %q", character)
		}
	}
	return normalized.String(), nil
}

func validCUITDigits(value string) bool {
	if len(value) != 11 {
		return false
	}
	weights := [...]int{5, 4, 3, 2, 7, 6, 5, 4, 3, 2}
	sum := 0
	for index, weight := range weights {
		sum += int(value[index]-'0') * weight
	}
	checkDigit := 11 - sum%11
	switch checkDigit {
	case 11:
		checkDigit = 0
	case 10:
		checkDigit = 9
	}
	return int(value[10]-'0') == checkDigit
}

func (cuit CUIT) String() string { return string(cuit) }

func (cuit CUIT) Int64() int64 {
	value, _ := strconv.ParseInt(string(cuit), 10, 64)
	return value
}

type ReceiverDocument struct {
	Type   DocumentType
	Number string
}

func NewReceiverDocument(documentType DocumentType, raw string) (ReceiverDocument, error) {
	value := strings.TrimSpace(raw)
	switch documentType {
	case DocumentCUIT, DocumentCUIL:
		cuit, err := ParseCUIT(value)
		if err != nil {
			return ReceiverDocument{}, err
		}
		return ReceiverDocument{Type: documentType, Number: cuit.String()}, nil
	case DocumentDNI:
		if len(value) < 7 || len(value) > 8 || !onlyDigits(value) {
			return ReceiverDocument{}, errors.New("DNI must contain 7 or 8 digits")
		}
	case DocumentConsumerFinal:
		if value == "" {
			value = "0"
		}
		if !onlyDigits(value) {
			return ReceiverDocument{}, errors.New("consumer final document must be numeric")
		}
	default:
		return ReceiverDocument{}, fmt.Errorf("unsupported receiver document type %d", documentType)
	}
	return ReceiverDocument{Type: documentType, Number: value}, nil
}

func ValidateReceiver(condition VATCondition, document ReceiverDocument) error {
	switch condition {
	case VATRegistered, VATMonotax, VATExempt:
		if document.Type != DocumentCUIT {
			return errors.New("registered, monotax, and exempt receivers require CUIT")
		}
	case VATConsumerFinal, VATNotResponsible:
		if document.Type != DocumentConsumerFinal && document.Type != DocumentCUIT &&
			document.Type != DocumentCUIL && document.Type != DocumentDNI {
			return errors.New("invalid receiver document")
		}
	default:
		return errors.New("invalid receiver VAT condition")
	}
	return nil
}

func onlyDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
