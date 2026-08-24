SET LOCAL lock_timeout='5s';
SET LOCAL statement_timeout='2min';

INSERT INTO admin_apis("group",path,method,description,sort_order,status,created_at,updated_at) VALUES
 ('运维监控','/admin/ops/compensation/drafts/:id/approve','POST','Explicitly approve Compensation draft',77,'active',NOW(),NOW()),
 ('运维监控','/admin/ops/compensation/drafts/:id/execute','POST','Resume approved Compensation execution',78,'active',NOW(),NOW()),
 ('运维监控','/admin/ops/compensation/execution-settings','PUT','Update Compensation execution permission',79,'active',NOW(),NOW()),
 ('运维监控','/admin/ops/compensation/executions/:id','GET','Read Compensation execution receipt',80,'active',NOW(),NOW()),
 ('运维监控','/admin/ops/compensation/erroneous-charge-refunds','POST','Create exact erroneous-charge reversal',81,'active',NOW(),NOW()),
 ('运维监控','/admin/ops/compensation/drafts/:id/execution-preview','GET','Preview Compensation execution gates',82,'active',NOW(),NOW())
ON CONFLICT(method,path) DO UPDATE SET "group"=EXCLUDED."group",description=EXCLUDED.description,sort_order=EXCLUDED.sort_order,status='active',updated_at=NOW();

INSERT INTO admin_role_apis(role_id,api_id,created_at)
SELECT rm.role_id,api.id,NOW() FROM admin_role_menus rm JOIN admin_menus m ON m.id=rm.menu_id AND m.permission_key='admin:ops' JOIN admin_apis api ON (api.method,api.path) IN(
 ('GET','/admin/ops/compensation/executions/:id'),('GET','/admin/ops/compensation/drafts/:id/execution-preview'))
ON CONFLICT(role_id,api_id) DO NOTHING;

INSERT INTO admin_role_apis(role_id,api_id,created_at)
SELECT role.id,api.id,NOW() FROM admin_roles role JOIN admin_apis api ON (api.method,api.path) IN(
 ('POST','/admin/ops/compensation/drafts/:id/approve'),('POST','/admin/ops/compensation/drafts/:id/execute'),('PUT','/admin/ops/compensation/execution-settings'),('POST','/admin/ops/compensation/erroneous-charge-refunds'))
WHERE role.is_super_admin=TRUE AND role.status='active'
ON CONFLICT(role_id,api_id) DO NOTHING;

DELETE FROM admin_role_apis relation USING admin_roles role,admin_apis api
WHERE relation.role_id=role.id AND relation.api_id=api.id AND role.is_super_admin=FALSE AND (api.method,api.path) IN(
 ('POST','/admin/ops/compensation/drafts/:id/approve'),('POST','/admin/ops/compensation/drafts/:id/execute'),('PUT','/admin/ops/compensation/execution-settings'),('POST','/admin/ops/compensation/erroneous-charge-refunds'));
