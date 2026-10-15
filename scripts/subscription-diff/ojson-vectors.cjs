// Regenerates backend-go/internal/subtpl/ojson/testdata/vectors.json:
//   node scripts/subscription-diff/ojson-vectors.cjs $UPSTREAM_DIR > backend-go/internal/subtpl/ojson/testdata/vectors.json
const U=process.argv[2]; const _=require(U+'/node_modules/lodash');
const cases=[];
for (const p of ['a.b[0].c',"a['b.c'].d",'a[0][1]','.a','a..b','a[]b','x["q\\"z"]']) cases.push({op:'toPath',path:p,out:_.toPath(p)});
const set=(obj,path,v)=>{const o=JSON.parse(JSON.stringify(obj)); _.set(o,path,v); return JSON.stringify(o);};
for (const [obj,path] of [[{},'a.b.c'],[{},'a[0].b'],[{a:5},'a.b'],[{a:[1]},'a[3]'],[{a:{x:1}},'a.y'],[{},'a.0.b']]) cases.push({op:'set',obj,path,out:set(obj,path,7)});
const unset=(obj,path)=>{const o=JSON.parse(JSON.stringify(obj)); _.unset(o,path); return JSON.stringify(o);};
for (const [obj,path] of [[{a:{b:1,c:2}},'a.b'],[{a:[1,2,3]},'a[1]'],[{a:1},'x.y']]) cases.push({op:'unset',obj,path,out:unset(obj,path)});
for (const v of [{a:"<&> ",b:[1,1.5,1e21,1e-7,-0.000001,null],c:{d:"\u0001\"\\"}}]) cases.push({op:'stringify',obj:v,out:JSON.stringify(v), out2:JSON.stringify(v,null,2)});
console.log(JSON.stringify(cases,null,1));
