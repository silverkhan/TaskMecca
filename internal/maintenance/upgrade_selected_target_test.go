package maintenance

import "testing"

func TestSelectedUpdateTargetMatchesChannelAndIsNewer(t *testing.T){
 tests:=[]struct{
  name,current,target,channel string
  allow bool
 }{
  {"dev_next","0.3.1-dev.29","0.3.1-dev.30","dev",true},
  {"dev_new_core","0.3.1-dev.29","0.3.2-dev.1","dev",true},
  {"stable_next","0.3.1","0.3.2","stable",true},
  {"already_installed","0.3.1-dev.29","0.3.1-dev.29","dev",false},
  {"dev_downgrade","0.3.1-dev.29","0.3.1-dev.28","dev",false},
  {"dev_to_stable","0.3.1-dev.29","0.3.2","dev",false},
  {"stable_to_dev","0.3.1","0.3.2-dev.1","stable",false},
  {"untrusted_suffix","0.3.1-dev.29","0.3.1-dev.30.evil","dev",false},
  {"url_path","0.3.1-dev.29","0.3.1-dev.30/../../latest","dev",false},
  {"empty","0.3.1-dev.29","","dev",false},
  {"tag_prefix","0.3.1-dev.29","v0.3.1-dev.30","dev",false},
  {"nul","0.3.1-dev.29","0.3.1-dev.30\x00","dev",false},
  {"alpha","0.3.1-dev.29","0.3.1-dev.abc","dev",false},
 }
 for _,tc:=range tests{
  t.Run(tc.name,func(t *testing.T){
   if got:=validSelectedUpgradeTarget(tc.current,tc.target,tc.channel);got!=tc.allow{
    t.Fatalf("target %q channel %s allow=%t got=%t",tc.target,tc.channel,tc.allow,got)
   }
  })
 }
}

func TestSelectedTargetInvalidRejectedBeforeNetworkAndInstall(t *testing.T){
 t.Setenv("TASK_MECCA_CHANNEL","dev")
 for _,target:=range []string{"../0.3.1","0.3.1-dev.1","0.3.1","0.3.1-dev.30/evil"}{
  _,err:=UpgradeWithSelectedTarget("0.3.1-dev.29",target,nil)
  if err==nil{t.Fatalf("invalid target %q accepted",target)}
 }
}
