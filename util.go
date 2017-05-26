package main

import (
	// "fmt"
	"reflect"
)

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

func containsKeyword(s []string, t ...string) bool {
	set := make(map[string]struct{})
	for _, v := range s {
		set[v] = struct{}{}
	}
	// fmt.Printf("set: %#v\n", set)
	// fmt.Printf("targets: %#v\n", t)
	for _, v := range t {
		if _, ok := set[v]; ok {
			// fmt.Printf("found target\n")
			return true
		}
	}
	return false
}
