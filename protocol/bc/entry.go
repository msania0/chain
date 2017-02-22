package bc

import (
	"fmt"
	"io"
	"reflect"

	"chain/crypto/sha3pool"
	"chain/encoding/blockchain"
	"chain/errors"
)

type (
	Entry interface {
		Type() string
		Body() interface{}
		Witness() interface{}
	}

	// EntryRef holds one or both of an entry and its id. If the entry
	// is present and the id is not, the id can be generated (and then
	// cached) on demand. Both may also be nil to represent a nil entry
	// pointer.
	EntryRef struct {
		Entry
		ID *Hash
	}
)

// Hash returns the EntryRef's cached entry ID, computing it first if
// necessary. Satisfies the hasher interface.
func (r *EntryRef) Hash() Hash {
	// xxx do we need to protect against concurrent calls to Hash()?
	if r.ID == nil {
		h := entryID(r.Entry)
		r.ID = &h
	}
	return *r.ID
}

func (r EntryRef) IsNil() bool {
	return r.Entry == nil && r.ID == nil
}

func (ref *EntryRef) writeEntry(w io.Writer) error {
	_, err := blockchain.WriteVarstr31(w, []byte(ref.Type()))
	if err != nil {
		return err
	}
	err = serialize(w, ref.Body())
	if err != nil {
		return err
	}
	return serialize(w, ref.Witness())
}

func (ref *EntryRef) readEntry(r io.Reader) error {
	typ, _, err := blockchain.ReadVarstr31(r)
	if err != nil {
		return err
	}
	switch string(typ) {
	case typeHeader:
		ref.Entry = new(Header)
	case typeIssuance:
		ref.Entry = new(Issuance)
	case typeMux:
		ref.Entry = new(mux)
	case typeNonce:
		ref.Entry = new(Nonce)
	case typeOutput:
		ref.Entry = new(Output)
	case typeRetirement:
		ref.Entry = new(Retirement)
	case typeSpend:
		ref.Entry = new(Spend)
	case typeTimeRange:
		ref.Entry = new(TimeRange)
	default:
		return fmt.Errorf("unknown type %s", typ)
	}
	body := ref.Entry.Body()
	err = deserialize(r, body)
	if err != nil {
		return err
	}
	witness := ref.Entry.Witness()
	return deserialize(r, witness)
}

var errInvalidValue = errors.New("invalid value")

func entryID(e Entry) Hash {
	h := sha3pool.Get256()
	defer sha3pool.Put256(h)

	h.Write([]byte("entryid:"))
	h.Write([]byte(e.Type()))
	h.Write([]byte{':'})

	bh := sha3pool.Get256()
	defer sha3pool.Put256(bh)
	err := serialize(bh, e.Body())
	if err != nil {
		panic(err) // xxx ok to panic here?
	}
	var innerHash Hash
	bh.Read(innerHash[:])
	h.Write(innerHash[:])

	var hash Hash
	h.Read(hash[:])

	return hash
}

func serializeEntry(w io.Writer, e Entry) error {
	err := serialize(w, e.Body())
	if err != nil {
		return err
	}
	return serialize(w, e.Witness())
}

func serialize(w io.Writer, c interface{}) (err error) {
	if c == nil {
		return nil
	}

	switch v := c.(type) {
	case byte:
		_, err = w.Write([]byte{v})
		return errors.Wrap(err, "writing byte")
	case uint64:
		_, err = blockchain.WriteVarint63(w, v)
		return errors.Wrapf(err, "writing uint64 (%d)", v)
	case []byte:
		_, err = blockchain.WriteVarstr31(w, v)
		return errors.Wrapf(err, "writing []byte (len %d)", len(v))
	case string:
		_, err = blockchain.WriteVarstr31(w, []byte(v))
		return errors.Wrapf(err, "writing string (len %d)", len(v))
	case EntryRef:
		h := v.Hash()
		_, err = w.Write(h[:])
		return errors.Wrap(err, "writing entryref hash")
	}

	// The two container types in the spec (List and Struct)
	// correspond to slices and structs in Go. They can't be
	// handled with type assertions, so we must use reflect.
	switch v := reflect.ValueOf(c); v.Kind() {
	case reflect.Ptr:
		// dereference and try again
		e := v.Elem()
		if !e.CanInterface() {
			return errInvalidValue
		}
		return serialize(w, e.Interface())

	case reflect.Array:
		elTyp := v.Type().Elem()
		if elTyp.Kind() != reflect.Uint8 {
			return errInvalidValue
		}
		// v is a fixed-length array of bytes
		_, err = w.Write(v.Slice(0, v.Len()).Bytes())
		return errors.Wrapf(err, "writing %d-byte array", v.Len())

	case reflect.Slice:
		l := v.Len()
		_, err := blockchain.WriteVarint31(w, uint64(l))
		if err != nil {
			return errors.Wrapf(err, "writing slice (len %d)", l)
		}
		for i := 0; i < l; i++ {
			c := v.Index(i)
			if !c.CanInterface() {
				return errInvalidValue
			}
			err := serialize(w, c.Interface())
			if err != nil {
				return errors.Wrapf(err, "writing slice element %d", i)
			}
		}
		return nil

	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			c := v.Field(i)
			if !c.CanInterface() {
				return errInvalidValue
			}
			err := serialize(w, c.Interface())
			if err != nil {
				t := v.Type()
				f := t.Field(i)
				return errors.Wrapf(err, "writing struct field %d (%s.%s)", i, t.Name(), f.Name)
			}
		}
		return nil
	}

	return errors.Wrap(fmt.Errorf("bad type %T", c))
}

func deserialize(r io.Reader, c interface{}) (err error) {
	if c == nil {
		return nil
	}

	switch v := c.(type) {
	case *byte:
		var b [1]byte
		_, err = r.Read(b[:])
		if err != nil {
			return errors.Wrap(err, "reading byte")
		}
		*v = b[0]
		return nil

	case *uint64:
		*v, _, err = blockchain.ReadVarint63(r)
		return errors.Wrap(err, "reading uint64")

	case *[]byte:
		*v, _, err = blockchain.ReadVarstr31(r)
		return errors.Wrap(err, "reading []byte")

	case *string:
		b, _, err := blockchain.ReadVarstr31(r)
		if err != nil {
			return errors.Wrap(err, "reading string")
		}
		*v = string(b)
		return nil
	case *EntryRef:
		var h Hash
		_, err = r.Read(h[:])
		if err != nil {
			return errors.Wrap(err, "reading hash for entryref")
		}
		v.ID = &h
		return nil
	}

	v := reflect.ValueOf(c)
	if v.Kind() != reflect.Ptr {
		return errInvalidValue
	}
	// v is *something
	switch elType := v.Type().Elem(); elType.Kind() {
	case reflect.Ptr:
		// v is **something
		// xxx

	case reflect.Slice:
		// v is *[]something
		n, _, err := blockchain.ReadVarint31(r)
		if err != nil {
			return errors.Wrap(err, "reading slice len")
		}
		slice := v.Elem()
		sliceElType := elType.Elem()
		for i := uint32(0); i < n; i++ {
			sliceElPtr := reflect.New(sliceElType)
			err = deserialize(r, sliceElPtr.Interface())
			if err != nil {
				return errors.Wrapf(err, "reading slice element %d", i)
			}
			slice = reflect.Append(slice, sliceElPtr.Elem())
		}
		v.Set(slice.Addr())
		return nil

	case reflect.Array:
		// v is *[...]something
		if elType.Elem().Kind() != reflect.Uint8 {
			return errInvalidValue
		}
		// c is *[...]byte
		b := make([]byte, 0, elType.Len())
		_, err = r.Read(b)
		if err != nil {
			return errors.Wrap(err, "reading %d-byte array", elType.Len())
		}
		reflect.Copy(v.Elem(), reflect.ValueOf(b))
		return nil

	case reflect.Struct:
		// v is *struct{ ... }
		s := v.Elem() // s is the struct
		for i := 0; i < v.NumField(); i++ {
			fPtr := s.Field(i).Addr()
			err = deserialize(r, fPtr.Interface())
			if err != nil {
				return errors.Wrapf(err, "reading struct field %d (%s.%s)", i, elType.Name(), elType.Field(i).Name)
			}
		}
		return nil
	}

	return errInvalidValue
}
