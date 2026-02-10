import{_ as u}from"./vendor-vue-BPZHirUR.js";/**
 * @license lucide-vue-next v0.474.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const w=t=>t.replace(/([a-z0-9])([A-Z])/g,"$1-$2").toLowerCase();/**
 * @license lucide-vue-next v0.474.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */var a={xmlns:"http://www.w3.org/2000/svg",width:24,height:24,viewBox:"0 0 24 24",fill:"none",stroke:"currentColor","stroke-width":2,"stroke-linecap":"round","stroke-linejoin":"round"};/**
 * @license lucide-vue-next v0.474.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const f=({size:t,strokeWidth:e=2,absoluteStrokeWidth:r,color:n,iconNode:s,name:o,class:i,...d},{slots:l})=>u("svg",{...a,width:t||a.width,height:t||a.height,stroke:n||a.stroke,"stroke-width":r?Number(e)*24/Number(t):e,class:["lucide",`lucide-${w(o??"icon")}`],...d},[...s.map(h=>u(...h)),...l.default?[l.default()]:[]]);/**
 * @license lucide-vue-next v0.474.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const p=(t,e)=>(r,{slots:n})=>u(f,{...r,iconNode:e,name:t},n);/**
 * @license lucide-vue-next v0.474.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const x=p("CircleAlertIcon",[["circle",{cx:"12",cy:"12",r:"10",key:"1mglay"}],["line",{x1:"12",x2:"12",y1:"8",y2:"12",key:"1pkeuh"}],["line",{x1:"12",x2:"12.01",y1:"16",y2:"16",key:"4dfq90"}]]);class y extends Error{constructor(e,r,n){super(`${e} ${r}`),this.status=e,this.statusText=r,this.body=n,this.name="ApiError"}}function g(){return window.location.origin}async function c(t,e,r){const n=`${g()}${e}`,s={};r!==void 0&&(s["Content-Type"]="application/json");const o=await fetch(n,{method:t,headers:s,body:r!==void 0?JSON.stringify(r):void 0});if(!o.ok){let i;try{i=await o.json()}catch{i=await o.text()}throw new y(o.status,o.statusText,i)}if(o.status!==204)return o.json()}function m(t){return c("GET",t)}function v(t,e){return c("POST",t,e)}function C(t,e){return c("PUT",t,e)}function E(t){return c("DELETE",t)}export{x as C,v as a,p as c,E as d,m as g,C as p};
