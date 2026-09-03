#!/usr/bin/env python3
"""Bounded direct-provider acceptance; never changes a customer/gateway key.

This is pre-release upstream evidence, NOT public gateway or billing proof.
Provider profiles bind an Agent Switch name to a fixed endpoint allowlist.
"""
from __future__ import annotations
import argparse
from dataclasses import asdict
import json
import os
from pathlib import Path
import subprocess
import sys
import urllib.request
import urllib.error

import model_release
import provider_contract_live_harness as harness

PROFILE = {
    'pomoai-gemini-normal': {
        'secret_name': 'LAOSHIRENAI_POMOAI_GEMINI_NORMAL_MAIN_KEY',
        'bases': {'hk2':'https://hk2.pomoai.xyz', 'jp':'https://jp.pomoai.xyz'},
    }
}


def secret_from_fd(name: str) -> str:
    r,w=os.pipe()
    try:
        proc=subprocess.Popen(['agent-switch','secret','get','--fd',str(w),name],pass_fds=(w,),stdin=subprocess.DEVNULL,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    finally:
        os.close(w)
    with os.fdopen(r,'rb') as reader:
        value=reader.read(16384)
    if proc.wait(timeout=15) != 0 or not value.strip():
        raise harness.HarnessError('Agent Switch credential unavailable')
    return value.decode().strip()


class DirectTransport:
    def __init__(self, base): self.base=base
    def post(self,url,headers,payload,timeout):
        if not url.startswith(self.base+'/'):
            raise harness.HarnessError('direct-provider URL outside approved profile')
        request=urllib.request.Request(url,data=json.dumps(payload).encode(),headers=headers,method='POST')
        try:
            with urllib.request.build_opener(harness.NoRedirect()).open(request,timeout=timeout) as response:
                body=response.read(8*1024*1024+1)
                if len(body)>8*1024*1024: raise harness.HarnessError('response too large')
                return harness.HTTPResult(response.status,dict(response.headers.items()),body,None)
        except urllib.error.HTTPError as error:
            return harness.HTTPResult(error.code,dict(error.headers.items()),error.read(1024*1024),None)
        except (urllib.error.URLError,TimeoutError,OSError) as error:
            return harness.HTTPResult(0,{},b'',{'type':type(error).__name__},isinstance(error,TimeoutError))


def main(argv=None):
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('command',choices=['plan','run'])
    p.add_argument('--manifest',type=Path,required=True)
    p.add_argument('--provider',choices=PROFILE,required=True)
    p.add_argument('--route',choices=['hk2','jp'],default='jp')
    p.add_argument('--protocol',choices=sorted(harness.PROTOCOLS),default='generate_content')
    p.add_argument('--cases',default='P-01,P-02,P-03,P-04,P-05,P-06,P-12,P-15')
    p.add_argument('--output-dir',type=Path,required=True)
    p.add_argument('--timeout',type=int,default=45)
    p.add_argument('--execute',action='store_true')
    p.add_argument('--acknowledge-paid-probes',action='store_true')
    a=p.parse_args(argv)
    try:
        if not 1<=a.timeout<=90: raise harness.HarnessError('timeout must be 1..90 seconds')
        manifest=json.loads(a.manifest.read_text());model_release.validate(manifest)
        model=manifest['model'];profile=PROFILE[a.provider];base=profile['bases'][a.route]
        if not model['id'].startswith('gemini-'): raise harness.HarnessError('profile restricted to Gemini model preflight')
        case_ids=a.cases.split(',')
        if set(case_ids)-harness.CASE_NAMES.keys(): raise harness.HarnessError('unknown case; context/resilience/billing require separate evidence')
        cases=[]
        for case_id in case_ids:
            efforts=manifest['capabilities']['reasoning_efforts'] if case_id=='P-06' else [None]
            for effort in efforts:
                key=f"upstream:{model['id']}:{a.protocol}:{case_id}:{effort or 'default'}"
                cases.append(harness.LiveCase(key,case_id,harness.CASE_NAMES[case_id],0,'direct-upstream',model['id'],a.protocol,base,'LSR_'+harness.digest_bytes(key.encode())[:12].upper(),reasoning_level=effort,context_window=model['context_window'],max_output_tokens=model['max_output_tokens']))
        if not cases: raise harness.HarnessError('no selected cases')
        plan={'kind':'direct_provider_plan','network_execution':'disabled','scope':'direct_upstream_only','model_id':model['id'],'base_url':base,'case_count':len(cases),'paid_request_upper_bound':sum(2 if c.p_id in {'P-05','P-07'} else 1 for c in cases),'cases':[asdict(c) for c in cases]}
        if a.command=='plan':
            harness.atomic_json(a.output_dir/'plan.json',plan)
            print(json.dumps({k:v for k,v in plan.items() if k!='cases'}));return 0
        if not a.execute or not a.acknowledge_paid_probes:
            raise harness.HarnessError('run requires --execute --acknowledge-paid-probes')
        # Never overwrite an earlier run's receipts; retry in a new directory.
        if a.output_dir.exists() and list(a.output_dir.glob('case-*.json')):
            raise harness.HarnessError('output directory already contains immutable receipts')
        key=secret_from_fd(profile['secret_name']);transport=DirectTransport(base);counts={}
        for index,case in enumerate(cases):
            started=harness.utc_now()
            shape,details,usage=harness.run_http_case(case,key,transport,a.timeout,'laoshirenai-provider-contract/direct')
            verification,reason=harness.verifier_observation(case,shape,usage,details)
            if case.p_id!='P-15' and (shape['http_status']!=200 or not shape['complete']):
                verification='failed';reason='requires complete HTTP 200'
            result='pass' if verification=='passed' else 'blocked'
            receipt=harness.with_artifact_sha({'schema_version':2,'kind':'provider_contract_live_case','scope':'direct_upstream_only','network_execution':'explicit_live','harness_sha256':harness.HARNESS_SHA256,'direct_runner_sha256':harness.digest_bytes(Path(__file__).read_bytes()),'case':asdict(case),'case_fingerprint':harness.digest_json(asdict(case)),'result':result,'classification':'verified' if result=='pass' else harness.blocked_classification(shape),'observed_at':started,'finished_at':harness.utc_now(),'offline_verifier':{'status':verification,'reason':reason},'response':shape,'usage':usage,'billing_attribution':{'status':'not_reconciled'},'secret_free':True})
            harness.atomic_json(a.output_dir/f'case-{index:02d}-{case.p_id}-{case.reasoning_level or "default"}.json',receipt)
            counts[result]=counts.get(result,0)+1
            print(json.dumps({'case':case.p_id,'effort':case.reasoning_level,'result':result,'http_status':shape['http_status'],'reason':reason}),flush=True)
        key=''
        summary={'scope':'direct_upstream_only','network_execution':'explicit_live','case_count':len(cases),'status_counts':counts,'production_configuration_changed':False,'public_gateway_verified':False,'billing_reconciled':False}
        harness.atomic_json(a.output_dir/'summary.json',summary)
        return 2 if counts.get('blocked') else 0
    except (ValueError,OSError) as error:
        # Error details must not echo a response, credential, or request headers.
        print(json.dumps({'error_type':type(error).__name__,'message':str(error) if isinstance(error,harness.HarnessError) else 'invalid input or runtime failure'}),file=sys.stderr)
        return 2

if __name__=='__main__':sys.exit(main())
