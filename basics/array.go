package basics

import (
	"encoding/json"
	"strings"
)

// ArrayInt ArrayInt64 可以解析数字, 数字字符串, 数字数组, 数字字符串数组
// 支持英文逗号分割解析成数组
type ArrayInt []int
type ArrayInt64 []int64

// ArrayKeyword 可以解析不包含英文逗号和空白符双引号的字符串
// 一般只用于解析只包含英文字母, 数字, 下划线, 中划线组成的字符串或字符串数组
// 支持英文逗号分割解析成数组
type ArrayKeyword []string

// ArrayString 解析任意字符串或字符串数组
type ArrayString []string

func (t *ArrayInt) UnmarshalJSON(b []byte) (err error) {
	s := strings.ReplaceAll(strings.Trim(string(b), "\" \r\n\t"), "\"", "")
	if s == "" || s == "null" {
		return
	}
	b = []byte(s)
	if b[0] != '[' {
		b = append([]byte{'['}, b...)
		b = append(b, ']')
	}
	var val []int
	err = json.Unmarshal(b, &val)
	*t = append(*t, val...)
	return
}

func (t *ArrayInt64) UnmarshalJSON(b []byte) (err error) {
	s := strings.ReplaceAll(strings.Trim(string(b), "\" \r\n\t"), "\"", "")
	if s == "" || s == "null" {
		return
	}
	b = []byte(s)
	if b[0] != '[' {
		b = append([]byte{'['}, b...)
		b = append(b, ']')
	}
	var val []int64
	err = json.Unmarshal(b, &val)
	*t = append(*t, val...)
	return
}

func (t *ArrayKeyword) UnmarshalJSON(b []byte) (err error) {
	s := strings.Trim(string(b), "[\" \r\n\t]")
	if s == "null" {
		return
	}
	ss := strings.Split(s, ",")
	for i, v := range ss {
		ss[i] = "\"" + strings.Trim(v, "[\" \r\n\t]") + "\""
	}
	s = "[" + strings.Join(ss, ",") + "]"
	var val []string
	err = json.Unmarshal([]byte(s), &val)
	*t = append(*t, val...)
	return
}

func (t *ArrayString) UnmarshalJSON(b []byte) (err error) {
	s := strings.Trim(string(b), "[\" \r\n\t]")
	if s == "null" {
		return
	}
	if b[0] == '[' {
		var val []string
		err = json.Unmarshal(b, &val)
		*t = append(*t, val...)
	} else {
		//s := strings.Trim(string(b), "\"")
		var val string
		err = json.Unmarshal(b, &val)
		*t = append(*t, val)
	}
	return
}
