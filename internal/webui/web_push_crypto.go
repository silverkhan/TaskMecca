package webui

import (
 "bytes"
 "crypto/aes"
 "crypto/cipher"
 "crypto/ecdh"
 "crypto/ecdsa"
 "crypto/elliptic"
 "crypto/hmac"
 "crypto/rand"
 "crypto/sha256"
 "encoding/asn1"
 "encoding/base64"
 "encoding/binary"
 "encoding/json"
 "errors"
 "fmt"
 "math/big"
 "net/http"
 "net/url"
 "time"
)

var pushB64 = base64.RawURLEncoding

// pushHKDF implements the RFC 5869 extract/expand with SHA-256. All callers
// use a single output block and bounded sizes.
func pushHKDF(salt,secret,info []byte,n int)[]byte{
 mac:=hmac.New(sha256.New,salt);_,_=mac.Write(secret);prk:=mac.Sum(nil)
 mac=hmac.New(sha256.New,prk);_,_=mac.Write(info);_,_=mac.Write([]byte{1})
 return mac.Sum(nil)[:n]
}

// RFC 8291 + RFC 8188 single-record aes128gcm content coding.
func encryptWebPush(uaPublic,authentication []byte,plaintext []byte)([]byte,error){
 if len(uaPublic)!=65||uaPublic[0]!=4||len(authentication)!=16 {return nil,errors.New("invalid browser push keys")}
 if len(plaintext)>3000 {return nil,errors.New("push payload too large")}
 pub,err:=ecdh.P256().NewPublicKey(uaPublic);if err!=nil{return nil,fmt.Errorf("invalid P-256 client key: %w",err)}
 priv,err:=ecdh.P256().GenerateKey(rand.Reader);if err!=nil{return nil,err}
 secret,err:=priv.ECDH(pub);if err!=nil{return nil,err}
 asPublic:=priv.PublicKey().Bytes()
 info:=append(append([]byte("WebPush: info\x00"),uaPublic...),asPublic...)
 ikm:=pushHKDF(authentication,secret,info,32)
 salt:=make([]byte,16);if _,err:=rand.Read(salt);err!=nil{return nil,err}
 key:=pushHKDF(salt,ikm,[]byte("Content-Encoding: aes128gcm\x00"),16)
 nonce:=pushHKDF(salt,ikm,[]byte("Content-Encoding: nonce\x00"),12)
 block,err:=aes.NewCipher(key);if err!=nil{return nil,err}
 gcm,err:=cipher.NewGCM(block);if err!=nil{return nil,err}
 // The record delimiter is 0x02 for the only (final) record.
 ciphertext:=gcm.Seal(nil,nonce,append(append([]byte{},plaintext...),0x02),nil)
 output:=bytes.NewBuffer(make([]byte,0,16+4+1+65+len(ciphertext)))
 output.Write(salt)
 _=binary.Write(output,binary.BigEndian,uint32(4096))
 output.WriteByte(byte(len(asPublic)))
 output.Write(asPublic)
 output.Write(ciphertext)
 if output.Len()>4096 {return nil,errors.New("encrypted push payload too large")}
 return output.Bytes(),nil
}

// vapidAuthorization is RFC 8292 ES256 and is deliberately bound to the
// push endpoint origin. The private key never leaves the Go server.
func vapidAuthorization(privateRaw []byte,publicB64 string,endpoint string)(string,error){
 if len(privateRaw)!=32{return "",errors.New("invalid VAPID private key")}
 parsed,err:=url.Parse(endpoint);if err!=nil||parsed.Scheme!="https"||parsed.Host=="" {return "",errors.New("invalid push endpoint")}
 x,y:=elliptic.Unmarshal(elliptic.P256(),mustPushDecode(publicB64))
 if x==nil||y==nil{return "",errors.New("invalid VAPID public key")}
 key:=&ecdsa.PrivateKey{PublicKey:ecdsa.PublicKey{Curve:elliptic.P256(),X:x,Y:y},D:new(big.Int).SetBytes(privateRaw)}
 header:=pushB64.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`))
 claims,_:=json.Marshal(map[string]any{"aud":parsed.Scheme+"://"+parsed.Host,"exp":time.Now().Add(12*time.Hour).Unix(),"sub":"https://github.com/silverkhan/TaskMecca"})
 unsigned:=header+"."+pushB64.EncodeToString(claims)
 digest:=sha256.Sum256([]byte(unsigned))
 sig,err:=ecdsa.SignASN1(rand.Reader,key,digest[:])
 if err!=nil{return "",err}
 r,s,err:=unmarshalVapidSignature(sig);if err!=nil{return "",err}
 signature:=append(r.FillBytes(make([]byte,32)),s.FillBytes(make([]byte,32))...)
 return "vapid t="+unsigned+"."+pushB64.EncodeToString(signature)+", k="+publicB64,nil
}
func mustPushDecode(s string)[]byte{b,_:=pushB64.DecodeString(s);return b}
func unmarshalVapidSignature(der []byte)(*big.Int,*big.Int,error){
 var decoded struct{ R,S *big.Int }
 // ASN.1 DER returned by ecdsa.SignASN1.
 rest,err:=parseVapidDER(der,&decoded)
 if err!=nil||len(rest)!=0||decoded.R==nil||decoded.S==nil{return nil,nil,errors.New("invalid ECDSA DER signature")}
 return decoded.R,decoded.S,nil
}
func parseVapidDER(data []byte,result *struct{ R,S *big.Int })([]byte,error){
 return asn1.Unmarshal(data,result)
}
func pushHTTPClient()*http.Client{
 return &http.Client{Timeout:8*time.Second,CheckRedirect:func(_ *http.Request,_ []*http.Request)error{return http.ErrUseLastResponse}}
}
