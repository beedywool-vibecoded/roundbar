package win

import "testing"

func TestParamIID(t *testing.T) {
	// Known instantiations, as listed in the Windows SDK headers.
	for sig, want := range map[string]string{
		SigGeneric("913337e9-11a1-4345-a3a2-4e7f956e222d", "string"): "98b9acc1-4b56-532e-ac73-03d5291cca90", // IVector<String>
		SigGeneric("faa585ea-6214-4217-afda-7f46de5869b3", "string"): "e2fcc7c1-3bfc-5a0b-b2b0-72e769d1cb7e", // IIterable<String>
	} {
		if got := ParamIID(sig); got != want {
			t.Errorf("%s: got %s, want %s", sig, got, want)
		}
	}
}
