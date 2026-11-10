const assert=require('node:assert/strict'),zlib=require('node:zlib');
function raster(png){
 let w,h,n,at=8,blocks=[];while(at<png.length){const l=png.readUInt32BE(at),kind=png.toString('ascii',at+4,at+8),b=png.subarray(at+8,at+8+l);at+=l+12;if(kind==='IHDR'){w=b.readUInt32BE(0);h=b.readUInt32BE(4);n={2:3,6:4}[b[9]];assert.ok(n)}if(kind==='IDAT')blocks.push(b)}
 const raw=zlib.inflateSync(Buffer.concat(blocks)),stride=w*n,pixels=Buffer.alloc(w*h*n);let prev=Buffer.alloc(stride),p=0;
 for(let y=0;y<h;y++){const filter=raw[p++],row=Buffer.from(raw.subarray(p,p+stride));p+=stride;for(let x=0;x<stride;x++){const a=x>=n?row[x-n]:0,b=prev[x],c=x>=n?prev[x-n]:0,v=a+b-c,d=[Math.abs(v-a),Math.abs(v-b),Math.abs(v-c)];row[x]=(row[x]+(filter===1?a:filter===2?b:filter===3?Math.floor((a+b)/2):filter===4?[a,b,c][d.indexOf(Math.min(...d))]:0))&255}row.copy(pixels,y*stride);prev=row}
 return{w,h,pixel:(x,y)=>[...pixels.subarray((Math.floor(y)*w+Math.floor(x))*n,(Math.floor(y)*w+Math.floor(x))*n+3)]};
}

module.exports={raster};
