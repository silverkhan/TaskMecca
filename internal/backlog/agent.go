package backlog

import (
    "regexp"
    "strings"
)

var agentPath = regexp.MustCompile(`^/root(?:/[a-z0-9_]+)*$`)

var workerPool = []string{
    "kkobugi","pairi","isanghaessi","pikachyu","raichyu","naong","jammanbo","ibui",
    "purin","metamong","mangnanyong","gorapadeok","paenteom","rukario","sikseuteil",
    "rapeuraseu","rioreu","togepi","seurakeu","eonibugi","geobukwang","rijadeu",
    "rijamong","isanghaepul","isanghaekkot","ppippi","myu","digeuda","kkoret",
    "moraeduji","kkomadol","rongseuton",
}

var workerPolicy = map[string][]string{
    "kkobugi":{"squirtle"}, "pairi":{"charmander"}, "isanghaessi":{"bulbasaur"},
    "pikachyu":{"pikachu"}, "raichyu":{}, "naong":{"meowth"},
    "jammanbo":{"snorlax"}, "ibui":{"eevee"}, "purin":{}, "metamong":{},
    "mangnanyong":{"dragonite"}, "gorapadeok":{"psyduck"}, "paenteom":{"gengar"},
    "rukario":{"lucario"}, "sikseuteil":{"vulpix"}, "rapeuraseu":{"lapras"},
    "rioreu":{"riolu"}, "togepi":{"togepi"}, "seurakeu":{"scyther"},
    "eonibugi":{}, "geobukwang":{}, "rijadeu":{}, "rijamong":{},
    "isanghaepul":{}, "isanghaekkot":{}, "ppippi":{}, "myu":{},
    "digeuda":{}, "kkoret":{}, "moraeduji":{}, "kkomadol":{}, "rongseuton":{},
}

func AgentReport(path string, newWorker bool) map[string]any {
    path = strings.TrimSpace(path)
    syntax := agentPath.MatchString(path)
    isWorker := strings.HasPrefix(path, "/root/controller/")
    var pokemon any = nil
    known := false
    allowedNew := false
    if isWorker {
        tail := strings.TrimPrefix(path, "/root/controller/")
        if !strings.Contains(tail, "/") {
            _, allowedNew = workerPolicy[tail]
            known = allowedNew
            if !known {
                for _, aliases := range workerPolicy {
                    for _, old := range aliases { if old == tail { known = true } }
                }
            }
        }
        pokemon = known
    }
    existing := syntax && (!isWorker || known)
    newOK := syntax && isWorker && allowedNew
    valid := existing
    mode := "existing_identity"
    if newWorker { valid = newOK; mode = "new_worker" }
    kind := "invalid"
    if syntax {
        kind = "subagent"
        if path == "/root" { kind = "root" } else if isWorker { kind = "worker" }
    }
    return map[string]any{
        "ok":valid, "path":path, "kind":kind, "path_syntax_ok":syntax,
        "pokemon_worker_name":pokemon, "existing_identity_ok":existing,
        "new_worker_ok":newOK, "validation_mode":mode,
        "rule":"new worker: agent <path> --new --json; existing identity/reuse: agent <path> --json",
        "legacy_note":"old identities stay readable and reusable; new creation uses only the confirmed ASCII pool, never rename an existing worker",
    }
}
