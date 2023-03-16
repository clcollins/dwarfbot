// file: pkg/dwarfbot/dwarfbot_test.go

/*
MIT License

Copyright (c) 2021 Chris Collins

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package dwarfbot

import (
	"reflect"
	"testing"
)

func TestDeduplicateChannels(t *testing.T) {
	testData := []struct {
		testName string
		input    []string
		expected []string
		output   []string
	}{
		{
			testName: "No Change",
			input:    []string{"apple", "banana", "pear"},
			expected: []string{"apple", "banana", "pear"},
		},
		{
			testName: "Deduplicate Apple",
			input:    []string{"apple", "banana", "pear", "apple"},
			expected: []string{"apple", "banana", "pear"},
		},
		{
			testName: "Deduplicate Apple and Sort",
			input:    []string{"banana", "apple", "pear", "apple"},
			expected: []string{"apple", "banana", "pear"},
		},
	}

	for _, data := range testData {
		data.output = deduplicateChannels(data.input)
		if !reflect.DeepEqual(data.output, data.expected) {
			t.Errorf("Deduplicate '%v' failed, got: '%v', want: '%v'", data.input, data.output, data.expected)
		}
	}
}
