package main

import "testing"

func Test_getExtFromUrl(t *testing.T) {
	type args struct {
		u string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"", args{"gemini://aaaa.com/"}, "noext"},
		{"", args{"gemini://aaaa.com/aaa/aaa.gmi"}, "gmi"},
		{"", args{"gemini://aaaa.com/aaa.jpg"}, "jpg"},
		{"", args{"////"}, "noext"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getExtFromUrl(tt.args.u); got != tt.want {
				t.Errorf("getExtFromUrl() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getUrlHash(t *testing.T) {
	type args struct {
		pageUrl string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"", args{"gemini://home.mkla.dev/pic.gif"}, "b3fed2f35340e6a87058b41f97ecabc3.gif"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getUrlHash(tt.args.pageUrl); got != tt.want {
				t.Errorf("getUrlHash() = %v, want %v", got, tt.want)
			}
		})
	}
}