#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';

const errors = [];
const fail = message => errors.push(message);
const root = path.resolve(process.argv[2] || '/out');
const gates = ['G1','G2','G3','G4','G5','G6','G7','G8'];
const states = new Set(['A','B','C','D','E']);
const scoreKeys = ['pain_magnitude_recurrence','evidence_strength','channel_accessibility','pilot_clarity','competitive_stagnation_whitespace','implementation_burden','continuing_manual_burden'];
const read = file => JSON.parse(fs.readFileSync(file, 'utf8'));
const sha = file => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const exists = rel => fs.existsSync(path.join(root, rel));
const requireFile = rel => { const file=path.join(root,rel); if(!fs.existsSync(file)||!fs.statSync(file).isFile()||!fs.statSync(file).size) fail(`missing/empty file: ${rel}`); return file; };
const requireDir = rel => { const dir=path.join(root,rel); if(!fs.existsSync(dir)||!fs.statSync(dir).isDirectory()) fail(`missing directory: ${rel}`); return dir; };
const jsonLines = rel => {
  const file=requireFile(rel); if(!fs.existsSync(file)) return [];
  return fs.readFileSync(file,'utf8').split(/\r?\n/).filter(Boolean).map((line,i)=>{try{return JSON.parse(line)}catch{fail(`${rel}:${i+1}: malformed JSONL`);return null}}).filter(Boolean);
};
const safeArtifact = rel => typeof rel==='string' && rel.length && !path.isAbsolute(rel) && !rel.split(/[\\/]/).includes('..');
const matches=(value,schema)=>{if(schema.const!==undefined&&JSON.stringify(value)!==JSON.stringify(schema.const))return false;if(schema.enum&&!schema.enum.includes(value))return false;if(schema.properties&&value&&typeof value==='object')for(const[k,s]of Object.entries(schema.properties))if(k in value&&!matches(value[k],s))return false;return true;};
const schemaCheck=(value,schema,where)=>{ if(schema.const!==undefined&&JSON.stringify(value)!==JSON.stringify(schema.const))fail(`${where}: const schema failure`); if(schema.enum&&!schema.enum.includes(value))fail(`${where}: enum schema failure`); for(const branch of schema.allOf||[])schemaCheck(value,branch,where); if(schema.if&&matches(value,schema.if))schemaCheck(value,schema.then||{},where); if(schema.if&&!matches(value,schema.if)&&schema.else)schemaCheck(value,schema.else,where); if(schema.type==='object'){if(!value||typeof value!=='object'||Array.isArray(value)){fail(`${where}: object schema failure`);return;}for(const key of schema.required||[])if(!(key in value))fail(`${where}: missing ${key}`);for(const [key,sub] of Object.entries(schema.properties||{}))if(key in value)schemaCheck(value[key],sub,`${where}.${key}`);if(schema.additionalProperties===false)for(const key of Object.keys(value))if(!(key in (schema.properties||{})))fail(`${where}: unexpected ${key}`);} if(schema.type==='array'){if(!Array.isArray(value)){fail(`${where}: array schema failure`);return;}if(schema.minItems!==undefined&&value.length<schema.minItems)fail(`${where}: too few items`);if(schema.maxItems!==undefined&&value.length>schema.maxItems)fail(`${where}: too many items`);if(schema.uniqueItems&&new Set(value.map(JSON.stringify)).size!==value.length)fail(`${where}: duplicate items`);for(const [i,item] of value.entries())if(schema.items)schemaCheck(item,schema.items,`${where}[${i}]`);} if(schema.type==='string'){if(typeof value!=='string')fail(`${where}: string schema failure`);if(schema.minLength!==undefined&&value.length<schema.minLength)fail(`${where}: too short`);}if(schema.type==='integer'){if(!Number.isInteger(value))fail(`${where}: integer schema failure`);if(schema.minimum!==undefined&&value<schema.minimum)fail(`${where}: below minimum`);if(schema.maximum!==undefined&&value>schema.maximum)fail(`${where}: above maximum`);}if(schema.pattern&&typeof value==='string'&&!new RegExp(schema.pattern).test(value))fail(`${where}: pattern schema failure`);};
const applySchema=(name,value,where)=>schemaCheck(value,read(path.resolve(path.dirname(new URL(import.meta.url).pathname),`../schemas/${name}.schema.json`)),where);
const citationResolves=(cite,base='')=>{if(typeof cite!=='string')return false;const m=cite.match(/^(.*?):(?:L?(\d+)|record-([\w-]+))$/);if(!m)return false;const rel=m[1];const options=[path.join(root,rel),path.join(root,base,rel)];const file=options.find(p=>p.startsWith(root+path.sep)&&fs.existsSync(p)&&fs.statSync(p).isFile());if(!file)return false;const physical=fs.readFileSync(file,'utf8').split(/\r?\n/);if(m[2])return Number(m[2])>=1&&Number(m[2])<=physical.length;if(m[3]){for(const line of physical.filter(Boolean))try{const record=JSON.parse(line);if(record.record_id===m[3]||record.candidate_id===m[3])return true;}catch{}return false;}return false;};

try {
  for (const rel of ['run-spec.json','gate-contract.json','execution-ledger.jsonl','candidates.jsonl','rejection-ledger.jsonl','handoff-manifest.json','REPORT.md','SUMMARY.md']) requireFile(rel);
  for (const rel of ['raw','territories','aggregate','finalists','selected','failures/quarantine']) requireDir(rel);
  const contractPath=path.join(root,'gate-contract.json'); const contract=read(contractPath);
  const canonicalContract=path.resolve(path.dirname(new URL(import.meta.url).pathname),'../assets/gate-contract-v1.json');
  if(sha(contractPath)!==sha(canonicalContract)) fail('gate-contract.json differs from bundled frozen contract');
  if(contract.contract_version!=='1.0.0'||contract.unit_of_analysis!=='first-value-workflow') fail('invalid contract identity');
  if(JSON.stringify(contract.evidence_states)!==JSON.stringify(['A','B','C','D','E'])) fail('invalid evidence states');
  if(JSON.stringify(contract.gates)!==JSON.stringify(gates)) fail('contract must contain ordered G1-G8');
  if(!Array.isArray(contract.gate_definitions)||new Set(contract.gate_definitions.map(x=>x.id)).size!==8||!gates.every(g=>contract.gate_definitions.some(x=>x.id===g))) fail('contract must define each gate exactly once');
  if(!Array.isArray(contract.hard_red_lines)||contract.hard_red_lines.length!==6) fail('contract must define six hard red lines');
  if(JSON.stringify(Object.keys(contract.scoring||{}).sort())!==JSON.stringify([...scoreKeys].sort())||Object.values(contract.scoring||{}).reduce((a,b)=>a+b,0)!==100) fail('invalid scoring contract');
  if(contract.targeted_job_limits?.round_1!==4||contract.targeted_job_limits?.round_2!==2||contract.targeted_job_limits?.total!==6||contract.retry_limit!==1||contract.unknown_is_disconfirmation!==false) fail('invalid bounds or unknown policy');
  applySchema('gate-contract',contract,'gate-contract.json');
  const spec=read(path.join(root,'run-spec.json')); if(spec.contract_sha256!==sha(contractPath)) fail('run-spec contract SHA-256 mismatch'); if(!Array.isArray(spec.raw_sources)||!spec.raw_sources.length)fail('run-spec raw_sources missing');for(const source of spec.raw_sources||[]){if(!source.original_path||!safeArtifact(source.delivered_path)||!source.sha256)fail('invalid raw source provenance');else{const file=requireFile(source.delivered_path);if(fs.existsSync(file)&&sha(file)!==source.sha256)fail(`raw source hash mismatch: ${source.delivered_path}`);}}
  const manifest=read(path.join(root,'handoff-manifest.json'));
  if(manifest.schema_version!=='1.0.0'||manifest.contract_sha256!==sha(contractPath)) fail('manifest identity/hash mismatch');
  if(manifest.next_skill!=='sandbox-hypothesis-stress-test'||manifest.mount_one_brief_per_run!==true) fail('downstream one-brief handoff policy missing');
  applySchema('handoff-manifest',manifest,'handoff-manifest.json');
  if(!Array.isArray(manifest.finalists)||!manifest.finalists.length) fail('finalists must be nonempty');
  const ledger=jsonLines('execution-ledger.jsonl');
  for(const [i,entry] of ledger.entries()) { if(!entry.artifact||!entry.action||!entry.status) fail(`execution ledger ${i+1}: missing artifact/action/status`); if(entry.execution_mode==='child'&&(!entry.job_id||entry.terminal_status!=='succeeded'||entry.exit_code!==0||!/^gpt-/.test(entry.model||'')||entry.no_descendant_delegation!==true)) fail(`execution ledger ${i+1}: child acceptance contract invalid`); if(entry.status==='accepted'&&entry.attempt>2) fail(`execution ledger ${i+1}: retry limit exceeded`);if(entry.status==='accepted'&&entry.sha256&&safeArtifact(entry.artifact)){const file=requireFile(entry.artifact);if(fs.existsSync(file)&&sha(file)!==entry.sha256)fail(`execution ledger ${i+1}: artifact hash mismatch`);} }
  const candidates=jsonLines('candidates.jsonl'); const candidateById=new Map();
  for(const [i,c] of candidates.entries()) {
    if(!c.candidate_id||!c.customer||!c.payer||!c.trigger_event||!c.first_value_workflow||!Array.isArray(c.citations)||!c.citations.length) fail(`candidate ${i+1}: required fields/citations missing`);
    if(JSON.stringify(Object.keys(c.scores||{}).sort())!==JSON.stringify([...scoreKeys].sort())) fail(`${c.candidate_id||i}: score dimensions differ from contract`);
    for(const [k,v] of Object.entries(c.scores||{})) if(!Number.isInteger(v)||v<0||v>5) fail(`${c.candidate_id}: invalid ${k} score`);
    const computed=Math.round(scoreKeys.reduce((sum,key)=>sum+(c.scores?.[key]||0)*contract.scoring[key]/5,0)); if(c.normalized_score!==undefined&&c.normalized_score!==computed) fail(`${c.candidate_id}: normalized score mismatch`); candidateById.set(c.candidate_id,{...c,computed});
    applySchema('candidate',c,`candidates.jsonl:${i+1}`); for(const cite of c.citations||[])if(!citationResolves(cite))fail(`${c.candidate_id}: unresolved candidate citation ${cite}`);
  }
  const seenFinalists=new Set();
  for(const finalist of manifest.finalists||[]) {
    const id=finalist.candidate_id; if(!id||seenFinalists.has(id)) { fail('missing/duplicate finalist id'); continue; } seenFinalists.add(id);
    const base=`finalists/${id}`; const indexPath=requireFile(`${base}/index.json`); requireFile(`${base}/problem-report.md`); requireDir(`${base}/avenues`); requireDir(`${base}/reviews`); requireDir(`${base}/gap-plans`); requireDir(`${base}/attempts`);
    if(!candidateById.has(id)) fail(`${id}: finalist missing from candidates.jsonl`); const candidate=candidateById.get(id);
    if(fs.existsSync(indexPath)){const index=read(indexPath); if(index.candidate_id!==id||index.contract_sha256!==sha(contractPath)||!Number.isInteger(index.normalized_score)||index.normalized_score!==candidate?.computed) fail(`${id}: invalid finalist index/score/hash`);}
    const covered=new Set(); for(const lane of ['customer','market','competitor','technical','risk']) { const rel=`${base}/avenues/${lane}.jsonl`; const findings=jsonLines(rel); if(!findings.length) fail(`${id}/${lane}: no findings`); for(const [i,f] of findings.entries()){applySchema('finding',f,`${rel}:${i+1}`);if(f.candidate_id!==id||f.lane!==lane)fail(`${rel}:${i+1}: finding identity failure`);for(const g of f.assigned_gates||[])covered.add(g);for(const cite of f.citations||[])if(!citationResolves(cite,base))fail(`${rel}:${i+1}: unresolved citation ${cite}`);} } if(!gates.every(g=>covered.has(g))) fail(`${id}: lane findings do not cover every assigned gate G1-G8`);
    const finalReviewPath=`${base}/reviews/final.json`; requireFile(finalReviewPath);
    if(exists(finalReviewPath)) {
      const review=read(path.join(root,finalReviewPath)); const reviewGates=(review.cells||[]).map(c=>c.gate);
      applySchema('gate-review',review,finalReviewPath);
      if(review.contract_sha256!==sha(contractPath)||reviewGates.length!==8||new Set(reviewGates).size!==8||!gates.every(g=>reviewGates.includes(g))) fail(`${id}: final review must contain exactly G1-G8`);
      for(const cell of review.cells||[]) { if(!states.has(cell.state)||!cell.basis) fail(`${id}/${cell.gate}: invalid state/basis`); if(['A','B'].includes(cell.state)&&(!Array.isArray(cell.citations)||!cell.citations.length)) fail(`${id}/${cell.gate}: A/B requires citation`); for(const cite of cell.citations||[]) if(!citationResolves(cite,base)) fail(`${id}/${cell.gate}: unresolvable citation ${cite}`); }
      if(review.normalized_score!==undefined&&review.normalized_score!==candidate?.computed)fail(`${id}: review/candidate score mismatch`);if(review.decision!==finalist.decision) fail(`${id}: review/manifest decision mismatch`); for(const cell of review.cells||[]) if(finalist.evidence_vector?.[cell.gate]!==cell.state) fail(`${id}/${cell.gate}: manifest/review state mismatch`); if(JSON.stringify(review.hard_red_line_findings||[])!==JSON.stringify(finalist.hard_red_line_findings||[])) fail(`${id}: hard-red-line findings mismatch`);
    }
    const actual={round_1:0,round_2:0}; for(const round of [1,2]) { const rel=`${base}/gap-plans/round-${round}.json`; requireFile(rel); if(exists(rel)){const plan=read(path.join(root,rel));applySchema('gap-plan',plan,rel);const jobs=plan.jobs||[]; actual[`round_${round}`]=jobs.length; if(plan.candidate_id!==id||plan.contract_sha256!==sha(contractPath)||plan.round!==round||jobs.length>(round===1?4:2)) fail(`${id}: round-${round} plan identity/hash/budget invalid`);for(const job of jobs)if(!/^gpt-/.test(job.model||'')||job.no_descendant_delegation!==true)fail(`${id}: invalid targeted job contract`);} }
    if(!['advance','evidence-insufficient','reject'].includes(finalist.decision)) fail(`${id}: invalid decision`);
    if(!finalist.evidence_vector||Object.keys(finalist.evidence_vector).sort().join(',')!==gates.join(',')||gates.some(g=>!states.has(finalist.evidence_vector[g]))) fail(`${id}: evidence vector must contain exactly G1-G8`);
    if((finalist.targeted_jobs?.round_1??99)>4||(finalist.targeted_jobs?.round_2??99)>2) fail(`${id}: targeted job budget exceeded`);
    if(finalist.targeted_jobs?.round_1!==actual.round_1||finalist.targeted_jobs?.round_2!==actual.round_2) fail(`${id}: manifest/plan targeted job counts mismatch`);
    if((finalist.unresolved_gaps||[]).some(gap=>typeof gap!=='string'||!gap.trim()||gap.includes('[object Object]'))) fail(`${id}: malformed unresolved gap`);
    if(!Array.isArray(finalist.hard_red_line_findings)) fail(`${id}: hard_red_line_findings must be an array`);
    for(const [kind,rel,expected] of [['report',finalist.report_path,finalist.report_sha256],...(finalist.decision==='advance'?[['brief',finalist.brief_path,finalist.brief_sha256]]:[])]) { if(!safeArtifact(rel)){fail(`${id}: unsafe ${kind} path`);continue;} const file=requireFile(rel); if(fs.existsSync(file)&&sha(file)!==expected) fail(`${id}: ${kind} SHA-256 mismatch`); }
    if(finalist.decision==='advance'&&(candidate.computed<contract.default_selection_policy.minimum_score||finalist.hard_red_line_findings.length||['G6','G7','G8'].some(g=>finalist.evidence_vector[g]!=='A')||Object.values(finalist.evidence_vector).includes('E'))) fail(`${id}: advance violates frozen decision policy`);
  }
  if(errors.length) { for(const error of errors) console.error(`INVALID: ${error}`); process.exitCode=1; } else console.log('VALID');
} catch (error) { console.error(`INVALID: ${error.message}`); process.exitCode=1; }
