import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).parents[1]
def load(name):
    spec = importlib.util.spec_from_file_location(name+'_safety', ROOT/'scripts'/f'{name}.py')
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module
H = load('provider_contract_live_harness')
R = load('provider_contract_runner')

class SafetyTest(unittest.TestCase):
    def case(self, p_id='P-06', protocol='generate_content'):
        return H.LiveCase('safety', p_id, 'test', 57, 'Gemini', 'gemini-3.8-flash', protocol,
                          'https://api.laoshirenai.com', 'MARKER', reasoning_level='high')

    def test_rejected_identity_never_writes_restore(self):
        class Controller:
            restored = False
            def open(self): raise H.HarnessError('identity mismatch')
            def restore(self): self.restored = True
        c=Controller()
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaises(H.HarnessError):
                H.run_cases([self.case()], Path(directory), timeout=1, controller_factory=lambda:c, transport_factory=lambda:object())
        self.assertFalse(c.restored)

    def test_gateway_key_never_reaches_arbitrary_origin(self):
        for url in ('http://api.laoshirenai.com', 'https://evil.test', 'https://api.laoshirenai.com.evil.test', 'https://x@api.laoshirenai.com', 'https://api.laoshirenai.com:444'):
            with self.assertRaises(H.HarnessError): H.approved_url(url)
        H.approved_url('https://api.laoshirenai.com/v1')
        with self.assertRaises(H.HarnessError): H.NoRedirect().redirect_request(None,None,302,None,None,'https://evil.test')

    def test_gemini_reasoning_wire_value_is_sent(self):
        payload=H.request_payload(self.case())
        self.assertEqual(payload['generationConfig']['thinkingConfig']['thinkingLevel'], 'high')

    def test_gemini_thought_tokens_are_in_output_usage(self):
        usage=H.extract_usage('generate_content', {'usageMetadata':{'promptTokenCount':10,'candidatesTokenCount':2,'thoughtsTokenCount':8,'totalTokenCount':20}})
        self.assertEqual(usage['output_tokens'],10)

    def test_plain_completion_does_not_prove_reasoning(self):
        state, _ = H.verifier_observation(self.case(), {'http_status':200,'complete':True}, None, {'observed_at':H.utc_now()})
        self.assertEqual(state,'failed')

    def test_html_error_does_not_prove_error_contract(self):
        state,_=H.verifier_observation(self.case('P-15'), {'http_status':502,'complete':False,'error':None},None,{'observed_at':H.utc_now()})
        self.assertEqual(state,'failed')

    def test_wrong_tool_and_non_json_arguments_do_not_pass(self):
        c=self.case('P-04','responses')
        for tool in ({'name':'wrong','arguments':{'value':'MARKER'},'call_id':'1'}, {'name':'echo_contract','arguments':'not JSON','call_id':'1'}, {'name':'echo_contract','arguments':{'value':'MARKER'}}):
            state,_=H.verifier_observation(c,{'http_status':200,'complete':True},None,{'observed_at':H.utc_now(),'tool_call':tool})
            self.assertEqual(state,'failed')

    def test_nan_is_not_a_number_for_billing(self):
        for value in (float('nan'),float('inf'),float('-inf'),True): self.assertIsNone(R._number(value))

    def test_forged_stale_fixture_cannot_resume(self):
        with tempfile.TemporaryDirectory() as directory:
            p=Path(directory)/'receipt.json'
            p.write_text(json.dumps(H.with_artifact_sha({'kind':'provider_contract_live_case','case_fingerprint':'x','result':'pass','observed_at':'2000-01-01T00:00:00Z','network_execution':'disabled'})))
            self.assertIsNone(H.verify_receipt(p,'x'))

    def test_fixture_evidence_remains_fixture_even_if_input_claims_live(self):
        ev=R._receipt_evidence({'evidence':{'source':'live_probe','id':'test'},'observed_at':H.utc_now()},'test-model')
        self.assertEqual(ev['kind'],'offline_fixture')

if __name__=='__main__': unittest.main()

class ProtocolProofTest(SafetyTest):
    def test_native_gemini_sse_joins_deltas_and_nested_terminal(self):
        events=[{'candidates':[{'content':{'parts':[{'text':'MAR'}]}}]}, {'candidates':[{'content':{'parts':[{'text':'KER'}]},'finishReason':'STOP'}]}]
        text,terminal=H.stream_result('generate_content',events)
        self.assertEqual((text,terminal),('MARKER','STOP'))
        bad=events+[{'error':{'message':'disconnected'}}]
        self.assertIsNone(H.stream_result('generate_content',bad)[1])

    def test_tool_result_is_not_disclosed_in_user_instructions(self):
        case=self.case('P-05','responses')
        payload=H.continuation_payload(case,{'output':[]},{'name':'echo_contract','call_id':'x'},None)
        marker=H.tool_result_marker(case)
        self.assertNotEqual(case.marker,marker)
        self.assertEqual(payload['input'][0]['output'],marker)
        user_messages=[x for x in payload['input'] if x.get('role')=='user']
        self.assertNotIn(marker,json.dumps(user_messages))

    def test_invalid_first_tool_does_not_trigger_second_request(self):
        case=self.case('P-05','responses')
        class Transport:
            calls=0
            def post(self,*args):
                self.calls+=1
                return H.HTTPResult(200,{},json.dumps({'status':'completed','output':[{'type':'function_call','name':'wrong','arguments':'not-json'}]}).encode(),None)
        transport=Transport()
        shape,details,usage=H.run_http_case(case,'synthetic-noncredential',transport,1,'fixture')
        self.assertEqual(transport.calls,1)
        self.assertIsNot(details.get('correlated'),True)

    def test_reasoning_configuration_echo_is_not_observed_reasoning(self):
        case=self.case('P-06','responses')
        class Transport:
            def post(self,*args):
                return H.HTTPResult(200,{},json.dumps({'status':'completed','output_text':'MARKER','reasoning':{'effort':'low','summary':None},'usage':{'input_tokens':1,'output_tokens':1,'total_tokens':2}}).encode(),None)
        shape,details,usage=H.run_http_case(case,'synthetic-noncredential',Transport(),1,'fixture')
        self.assertIsNone(details['reasoning'])
        self.assertEqual(H.verifier_observation(case,shape,usage,details)[0],'failed')
