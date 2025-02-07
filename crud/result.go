package crud

import (
	"fmt"
	"reflect"

	"github.com/vmihailenco/msgpack/v5"
	"github.com/vmihailenco/msgpack/v5/msgpcode"
)

// FieldFormat contains field definition: {name='...',type='...'[,is_nullable=...]}.
type FieldFormat struct {
	Name       string
	Type       string
	IsNullable bool
}

// DecodeMsgpack provides custom msgpack decoder.
func (format *FieldFormat) DecodeMsgpack(d *msgpack.Decoder) error {
	l, err := d.DecodeMapLen()
	if err != nil {
		return err
	}
	for i := 0; i < l; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		switch key {
		case "name":
			if format.Name, err = d.DecodeString(); err != nil {
				return err
			}
		case "type":
			if format.Type, err = d.DecodeString(); err != nil {
				return err
			}
		case "is_nullable":
			if format.IsNullable, err = d.DecodeBool(); err != nil {
				return err
			}
		default:
			if err := d.Skip(); err != nil {
				return err
			}
		}
	}

	return nil
}

// Result describes CRUD result as an object containing metadata and rows.
type Result struct {
	Metadata []FieldFormat
	Rows     interface{}
	rowType  reflect.Type
}

// MakeResult create a Result object with a custom row type for decoding.
func MakeResult(rowType reflect.Type) Result {
	return Result{
		rowType: rowType,
	}
}

func msgpackIsArray(code byte) bool {
	return code == msgpcode.Array16 || code == msgpcode.Array32 ||
		msgpcode.IsFixedArray(code)
}

// DecodeMsgpack provides custom msgpack decoder.
func (r *Result) DecodeMsgpack(d *msgpack.Decoder) error {
	arrLen, err := d.DecodeArrayLen()
	if err != nil {
		return err
	}

	if arrLen == 0 {
		return fmt.Errorf("unexpected empty response array")
	}

	// DecodeMapLen processes `nil` as zero length map,
	// so in `return nil, err` case we don't miss error info.
	// https://github.com/vmihailenco/msgpack/blob/3f7bd806fea698e7a9fe80979aa3512dea0a7368/decode_map.go#L79-L81
	l, err := d.DecodeMapLen()
	if err != nil {
		return err
	}

	for i := 0; i < l; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}

		switch key {
		case "metadata":
			metadataLen, err := d.DecodeArrayLen()
			if err != nil {
				return err
			}

			metadata := make([]FieldFormat, metadataLen)

			for i := 0; i < metadataLen; i++ {
				fieldFormat := FieldFormat{}
				if err = d.Decode(&fieldFormat); err != nil {
					return err
				}

				metadata[i] = fieldFormat
			}

			r.Metadata = metadata
		case "rows":
			if r.rowType != nil {
				tuples := reflect.New(reflect.SliceOf(r.rowType))
				if err = d.DecodeValue(tuples); err != nil {
					return err
				}
				r.Rows = tuples.Elem().Interface()
			} else {
				var decoded []interface{}
				if err = d.Decode(&decoded); err != nil {
					return err
				}
				r.Rows = decoded
			}
		default:
			if err := d.Skip(); err != nil {
				return err
			}
		}
	}

	if arrLen > 1 {
		code, err := d.PeekCode()
		if err != nil {
			return err
		}

		if msgpackIsArray(code) {
			crudErr := newErrorMany(r.rowType)
			if err := d.Decode(&crudErr); err != nil {
				return err
			}
			if crudErr != nil {
				return *crudErr
			}
		} else if code != msgpcode.Nil {
			crudErr := newError(r.rowType)
			if err := d.Decode(&crudErr); err != nil {
				return err
			}
			if crudErr != nil {
				return *crudErr
			}
		} else {
			if err := d.DecodeNil(); err != nil {
				return err
			}
		}
	}

	for i := 2; i < arrLen; i++ {
		if err := d.Skip(); err != nil {
			return err
		}
	}

	return nil
}

// NumberResult describes CRUD result as an object containing number.
type NumberResult struct {
	Value uint64
}

// DecodeMsgpack provides custom msgpack decoder.
func (r *NumberResult) DecodeMsgpack(d *msgpack.Decoder) error {
	arrLen, err := d.DecodeArrayLen()
	if err != nil {
		return err
	}

	if arrLen == 0 {
		return fmt.Errorf("unexpected empty response array")
	}

	// DecodeUint64 processes `nil` as `0`,
	// so in `return nil, err` case we don't miss error info.
	// https://github.com/vmihailenco/msgpack/blob/3f7bd806fea698e7a9fe80979aa3512dea0a7368/decode_number.go#L91-L93
	if r.Value, err = d.DecodeUint64(); err != nil {
		return err
	}

	if arrLen > 1 {
		var crudErr *Error = nil

		if err := d.Decode(&crudErr); err != nil {
			return err
		}

		if crudErr != nil {
			return crudErr
		}
	}

	for i := 2; i < arrLen; i++ {
		if err := d.Skip(); err != nil {
			return err
		}
	}

	return nil
}

// BoolResult describes CRUD result as an object containing bool.
type BoolResult struct {
	Value bool
}

// DecodeMsgpack provides custom msgpack decoder.
func (r *BoolResult) DecodeMsgpack(d *msgpack.Decoder) error {
	arrLen, err := d.DecodeArrayLen()
	if err != nil {
		return err
	}

	if arrLen == 0 {
		return fmt.Errorf("unexpected empty response array")
	}

	// DecodeBool processes `nil` as `false`,
	// so in `return nil, err` case we don't miss error info.
	// https://github.com/vmihailenco/msgpack/blob/3f7bd806fea698e7a9fe80979aa3512dea0a7368/decode.go#L367-L369
	if r.Value, err = d.DecodeBool(); err != nil {
		return err
	}

	if arrLen > 1 {
		var crudErr *Error = nil

		if err := d.Decode(&crudErr); err != nil {
			return err
		}

		if crudErr != nil {
			return crudErr
		}
	}

	for i := 2; i < arrLen; i++ {
		if err := d.Skip(); err != nil {
			return err
		}
	}

	return nil
}
