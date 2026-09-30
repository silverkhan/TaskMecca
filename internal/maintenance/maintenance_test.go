package maintenance

import "testing"

func TestCompareVersions(t *testing.T) {
    cases:=[]struct{
        latest string
        current string
        want bool
    }{
        {"0.2.10","0.2.9",true},
        {"0.3.0","0.2.99",true},
        {"1.0.0","0.9.9",true},
        {"0.2.9","0.2.9",false},
        {"0.2.8","0.2.9",false},
        {"v0.2.10","0.2.9",true},
    }
    for _,tc:=range cases {
        if got:=newerVersion(tc.latest,tc.current); got!=tc.want {
            t.Fatalf("newerVersion(%q,%q)=%v want %v",tc.latest,tc.current,got,tc.want)
        }
    }
}
