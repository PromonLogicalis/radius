package radius

import (
	"errors"
	"sync"
)

var builtinOnce sync.Once

// Builtin is the built-in dictionary. It is initially loaded with the
// attributes defined in RFC 2865 and RFC 2866.
var Builtin *Dictionary

func initDictionary() {
	Builtin = &Dictionary{}
}

// DictionaryEntry stores a single mapping between an attribute name, type and
// AttributeCodec.
type DictionaryEntry struct {
	Type  byte
	Name  string
	Codec AttributeCodec
}

// Dictionary stores mappings between attribute names and types and
// AttributeCodecs.
type Dictionary struct {
	mu               sync.RWMutex
	attributesByType [256]*DictionaryEntry
	attributesByName map[string]*DictionaryEntry
}

// Register registers the AttributeCodec for the given attribute name and type.
func (d *Dictionary) Register(name string, t byte, codec AttributeCodec) error {
	d.mu.Lock()
	if d.attributesByType[t] != nil {
		d.mu.Unlock()
		return errors.New("radius: attribute already registered")
	}
	entry := &DictionaryEntry{
		Type:  t,
		Name:  name,
		Codec: codec,
	}
	d.attributesByType[t] = entry
	if d.attributesByName == nil {
		d.attributesByName = make(map[string]*DictionaryEntry)
	}
	d.attributesByName[name] = entry
	d.mu.Unlock()
	return nil
}

// MustRegister is a helper for Register that panics if it returns an error.
func (d *Dictionary) MustRegister(name string, t byte, codec AttributeCodec) {
	if err := d.Register(name, t, codec); err != nil {
		panic(err)
	}
}

func (d *Dictionary) get(name string) (t byte, codec AttributeCodec, ok bool) {
	d.mu.RLock()
	entry := d.attributesByName[name]
	d.mu.RUnlock()
	if entry == nil {
		return
	}
	t = entry.Type
	codec = entry.Codec
	ok = true
	return
}

// Remove removes an attribute from the dictionary by type. It returns an error
// only if the attribute type does not exist.
func (d *Dictionary) Remove(t byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry := d.attributesByType[t]
	if entry == nil {
		return errors.New("radius: attribute is not registered")
	}
	d.attributesByType[t] = nil
	delete(d.attributesByName, entry.Name)
	return nil
}

// RemoveByName removes an attribute from the dictionary by name. It returns an
// error only if the attribute name does not exist.
func (d *Dictionary) RemoveByName(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry, ok := d.attributesByName[name]
	if !ok {
		return errors.New("radius: attribute is not registered")
	}
	d.attributesByType[entry.Type] = nil
	delete(d.attributesByName, name)
	return nil
}

// Attr returns a new *Attribute whose type is registered under the given
// name.
//
// If name is not registered, nil and an error is returned.
//
// If the attribute's codec implements AttributeTransformer, the value is
// first transformed before being stored in *Attribute. If the transform
// function returns an error, nil and the error is returned.
func (d *Dictionary) Attr(name string, value interface{}) (*Attribute, error) {
	t, codec, ok := d.get(name)
	if !ok {
		return nil, errors.New("radius: attribute name not registered")
	}
	if transformer, ok := codec.(AttributeTransformer); ok {
		transformed, err := transformer.Transform(value)
		if err != nil {
			return nil, err
		}
		value = transformed
	}
	return &Attribute{
		Type:  t,
		Value: value,
	}, nil
}

// MustAttr is a helper for Attr that panics if Attr were to return an error.
func (d *Dictionary) MustAttr(name string, value interface{}) *Attribute {
	attr, err := d.Attr(name, value)
	if err != nil {
		panic(err)
	}
	return attr
}

// Name returns the registered name for the given attribute type. ok is false
// if the given type is not registered.
func (d *Dictionary) Name(t byte) (name string, ok bool) {
	d.mu.RLock()
	entry := d.attributesByType[t]
	d.mu.RUnlock()
	if entry == nil {
		return
	}
	name = entry.Name
	ok = true
	return
}

// Type returns the registered type for the given attribute name. ok is false
// if the given name is not registered.
func (d *Dictionary) Type(name string) (t byte, ok bool) {
	d.mu.RLock()
	entry := d.attributesByName[name]
	d.mu.RUnlock()
	if entry == nil {
		return
	}
	t = entry.Type
	ok = true
	return
}

// Codec returns the AttributeCodec for the given registered type. nil is
// returned if the given type is not registered.
func (d *Dictionary) Codec(t byte) AttributeCodec {
	d.mu.RLock()
	entry := d.attributesByType[t]
	d.mu.RUnlock()
	if entry == nil {
		return AttributeUnknown
	}
	return entry.Codec
}
