SET LOCAL lock_timeout='5s';
SET LOCAL statement_timeout='2min';

INSERT INTO admin_apis("group",path,method,description,sort_order,status,created_at,updated_at) VALUES
 ('运维监控','/admin/ops/compensation','GET','Compensation Shadow overview',71,'active',NOW(),NOW()),
 ('运维监控','/admin/ops/compensation/drafts/:id','GET','Compensation Shadow draft explanation',72,'active',NOW(),NOW()),
 ('运维监控','/admin/ops/compensation/incidents/:id/draft','POST','Generate Compensation Shadow draft',73,'active',NOW(),NOW()),
 ('运维监控','/admin/ops/compensation/drafts/:id/revisions','POST','Create immutable Compensation draft revision',74,'active',NOW(),NOW()),
 ('运维监控','/admin/ops/compensation/settings','PUT','Update Compensation Shadow evaluator',75,'active',NOW(),NOW()),
 ('运维监控','/admin/ops/compensation/drafts/:id/reviews','POST','Record Compensation Shadow comparison',76,'active',NOW(),NOW())
ON CONFLICT(method,path) DO UPDATE SET "group"=EXCLUDED."group",description=EXCLUDED.description,sort_order=EXCLUDED.sort_order,status='active',updated_at=NOW();

INSERT INTO admin_role_apis(role_id,api_id,created_at)
SELECT rm.role_id,api.id,NOW() FROM admin_role_menus rm JOIN admin_menus m ON m.id=rm.menu_id AND m.permission_key='admin:ops' JOIN admin_apis api ON (api.method,api.path) IN(
 ('GET','/admin/ops/compensation'),('GET','/admin/ops/compensation/drafts/:id'),('POST','/admin/ops/compensation/incidents/:id/draft'),('POST','/admin/ops/compensation/drafts/:id/revisions'),('PUT','/admin/ops/compensation/settings'),('POST','/admin/ops/compensation/drafts/:id/reviews'))
ON CONFLICT(role_id,api_id) DO NOTHING;
