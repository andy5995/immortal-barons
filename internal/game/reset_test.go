package game

import (
	"reflect"
	"testing"
)

// fillAll sets every exported field reachable from v to a non-zero value, one
// level deep into a self-containing type, so a reset that kept something it
// should not cannot hide behind a field that was zero anyway.
func fillAll(v reflect.Value, path map[reflect.Type]bool) {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(7)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(7)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(7)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.String:
		v.SetString("x")
	case reflect.Slice:
		el := reflect.New(v.Type().Elem()).Elem()
		fillAll(el, path)
		v.Set(reflect.Append(reflect.MakeSlice(v.Type(), 0, 1), el))
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		k, e := reflect.New(v.Type().Key()).Elem(), reflect.New(v.Type().Elem()).Elem()
		fillAll(k, path)
		fillAll(e, path)
		m.SetMapIndex(k, e)
		v.Set(m)
	case reflect.Pointer:
		if path[v.Type().Elem()] {
			return
		}
		v.Set(reflect.New(v.Type().Elem()))
		fillAll(v.Elem(), path)
	case reflect.Struct:
		if path[v.Type()] {
			return
		}
		path[v.Type()] = true
		defer delete(path, v.Type())
		for i := range v.NumField() {
			if v.Type().Field(i).IsExported() {
				fillAll(v.Field(i), path)
			}
		}
	}
}

// A reset erases every field the save holds, including one added after this
// test was written, and brings it back exactly as a brand-new world has it.
// The fields the save does not hold (json:"-") are read from their own files
// and are left alone.
func TestResetErasesEverythingSaved(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Pirates = false // seeding draws the RNG, which the fill leaves elsewhere
	fresh := NewWorldSeed(cfg, 1)

	w := NewWorldSeed(cfg, 1)
	v := reflect.ValueOf(w).Elem()
	for i := range v.NumField() {
		if v.Type().Field(i).IsExported() {
			fillAll(v.Field(i), map[reflect.Type]bool{})
		}
	}
	w.Config = cfg
	unsaved := map[string]any{}
	for i := range v.NumField() {
		if f := v.Type().Field(i); f.IsExported() && f.Tag.Get("json") == "-" {
			unsaved[f.Name] = v.Field(i).Interface()
		}
	}
	if len(unsaved) == 0 {
		t.Fatal("no World field is tagged json:\"-\"; the test is not looking at what it should")
	}
	w.Reset()

	fv, after := reflect.ValueOf(fresh).Elem(), reflect.ValueOf(w).Elem()
	for i := range after.NumField() {
		f := after.Type().Field(i)
		if !f.IsExported() {
			continue
		}
		got := after.Field(i).Interface()
		if want, ok := unsaved[f.Name]; ok {
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s is not in the save, so a reset must leave it alone, but it changed", f.Name)
			}
			continue
		}
		if want := fv.Field(i).Interface(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s survived a reset: %v, a new world has %v", f.Name, got, want)
		}
	}
}
