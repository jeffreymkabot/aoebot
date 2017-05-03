package main

import (
	"reflect"
)

func containsKeyword(s []string, t ...string) bool {
	set := make(map[string]bool)
	for _, v := range s {
		set[v] = true
	}
	for _, v := range t {
		if set[v] {
			return true
		}
	}
	return false
}

// http://stackoverflow.com/questions/10485743/contains-method-for-a-slice
func containsAny(s interface{}, t ...interface{}) bool {
	sSlice := reflect.ValueOf(s)
	if sSlice.Kind() == reflect.Slice {
		set := make(map[interface{}]struct{}, sSlice.Len())
		for i := 0; i < sSlice.Len(); i++ {
			set[sSlice.Index(i).Interface()] = struct{}{}
		}
		for _, v := range t {
			if _, ok := set[v]; ok {
				return true
			}
		}
	}
	return false
}