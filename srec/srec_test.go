package srec

import "testing"

func TestParseGeneric(t *testing.T) {
	test := "S030000047656E6572617465642066726F6D206669726D776172655F7061636B6167652E622062792065626D2D7574696CFA"
	typ, data, err := ParseGeneric(test)
	if err != nil {
		t.Error(err)
	}
	if typ != 0 {
		t.Errorf("wrong typ, expected 0, got %d", typ)
	}
	t.Log(string(data))
}
