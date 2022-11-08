package main

import "fmt"

type strValue struct {
	s     *string
	isSet bool
}

func (v *strValue) String() string {
	if v.s != nil {
		return *v.s
	}
	return ""
}

func (v *strValue) Set(s string) error {
	//panic("what")
	fmt.Printf("before value %s\n", *v.s)
	*v.s = s
	v.isSet = true
	fmt.Printf("set value %s\n", *v.s)
	return nil
}
