from datetime import date
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

from scripts.model_evidence_integrity import artifact_gaps

ROOT=Path(__file__).parents[1]

class GateTest(unittest.TestCase):
    def test_empty_public_inventory_does_not_vacuously_pass(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d); (root/'inventory.json').write_text('{"models":[]}')
            (root/'clients.json').write_text('{"clients":[]}')
            result=subprocess.run([sys.executable,str(ROOT/'scripts/model_doc_matrix.py'),'audit','--contracts',str(root),'--client-matrix',str(root/'clients.json'),'--inventory-json',str(root/'inventory.json'),'--require-model','gemini-3.8-flash'],capture_output=True,text=True)
            self.assertEqual(result.returncode,2,result.stderr)
            report=json.loads(result.stdout)
            self.assertFalse(report['complete'])
            self.assertTrue(any('empty' in f for f in report['failures']))
            self.assertTrue(any('required model' in f for f in report['failures']))

    def test_terminal_evidence_requires_actual_matching_bytes(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);p=root/'live.json';p.write_text('{"kind":"official_model_spec","url":"https://example.test/model","observed_at":"2026-09-03"}')
            ev={'status':'verified','source_ref':'live.json','artifact_sha256':hashlib.sha256(p.read_bytes()).hexdigest(),'observed_at':'2026-09-03'}
            self.assertEqual(artifact_gaps(ev,root,as_of=date(2026,9,3)),[])
            p.write_text('{}')
            self.assertTrue(any('digest mismatch' in f for f in artifact_gaps(ev,root,as_of=date(2026,9,3))))

    def test_fixture_never_becomes_live_by_hashing_it(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);p=root/'fixture.json';p.write_text('{"kind":"provider_contract_live_case","network_execution":"disabled"}')
            ev={'status':'verified','source_ref':'fixture.json','artifact_sha256':hashlib.sha256(p.read_bytes()).hexdigest(),'observed_at':'2026-09-03'}
            self.assertTrue(any('offline fixture' in f for f in artifact_gaps(ev,root,as_of=date(2026,9,3))))

    def test_missing_stale_future_and_escaping_evidence_fail(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d)
            for ref,observed in [('missing.json','2026-09-03'),('../escape.json','2026-09-03'),('missing.json','2020-01-01'),('missing.json','2027-01-01')]:
                self.assertTrue(artifact_gaps({'status':'verified','source_ref':ref,'observed_at':observed,'artifact_sha256':'0'*64},root,as_of=date(2026,9,3)))

    def test_matrix_schema_has_exactly_the_required_nine_dimensions(self):
        schema=json.loads((ROOT/'model-doc-contracts/matrix-schema.json').read_text())
        self.assertEqual(len(schema['properties']['matrices']['required']),9)

if __name__=='__main__': unittest.main()

class CatalogMutationGateTest(unittest.TestCase):
    def test_draft_apply_refused_before_catalog_or_generated_files_change(self):
        with tempfile.TemporaryDirectory() as d:
            catalog=Path(d)/'catalog.json'
            original=(ROOT/'model-catalog/catalog.json').read_bytes();catalog.write_bytes(original)
            result=subprocess.run([sys.executable,str(ROOT/'scripts/model_catalog.py'),'apply','--catalog',str(catalog),'--manifest',str(ROOT/'model-doc-contracts/releases/gemini-3.8-flash.draft.json')],capture_output=True,text=True)
            self.assertEqual(result.returncode,2)
            self.assertIn('not ready',result.stderr)
            self.assertEqual(catalog.read_bytes(),original)

    def test_blocked_provider_receipt_cannot_satisfy_verified_wrapper(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);p=root/'source.json'
            p.write_text(json.dumps({'schema_version':2,'kind':'provider_contract_live_case','network_execution':'explicit_live','observed_at':'2026-09-03','result':'blocked','classification':'usage_attribution_missing','case':{'model_id':'test'},'offline_verifier':{'status':'passed'},'response':{'http_status':200}}))
            ev={'status':'verified','source_ref':'source.json','artifact_sha256':hashlib.sha256(p.read_bytes()).hexdigest(),'observed_at':'2026-09-03'}
            self.assertTrue(any('incomplete live' in f for f in artifact_gaps(ev,root,as_of=date(2026,9,3))))

class DirectHarnessGatesTest(unittest.TestCase):
    def test_plan_is_offline_and_paid_execution_requires_both_flags(self):
        with tempfile.TemporaryDirectory() as d:
            args=[sys.executable,str(ROOT/'scripts/provider_contract_direct.py'),'plan','--manifest',str(ROOT/'model-doc-contracts/releases/gemini-3.8-flash.draft.json'),'--provider','pomoai-gemini-normal','--cases','P-01','--output-dir',d]
            planned=subprocess.run(args,capture_output=True,text=True)
            self.assertEqual(planned.returncode,0,planned.stderr)
            self.assertEqual(json.loads(planned.stdout)['network_execution'],'disabled')
            args[2]='run'
            denied=subprocess.run(args,capture_output=True,text=True)
            self.assertEqual(denied.returncode,2)
            self.assertIn('requires --execute',denied.stderr)

    def test_locked_client_snapshot_matches_provenance(self):
        lock=json.loads((ROOT/'model-doc-contracts/import-provenance.json').read_text())
        expected=next(x['source_sha256'] for x in lock['files'] if x['path']=='model-doc-contracts/client-matrix.json')
        self.assertEqual(hashlib.sha256((ROOT/'model-doc-contracts/client-matrix.json').read_bytes()).hexdigest(),expected)
