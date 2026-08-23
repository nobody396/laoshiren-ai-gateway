-- Grant Incident Control read/editor APIs to roles already carrying the
-- admin:ops menu permission. Public timeline remains unauthenticated and is
-- protected by its own default-off visibility switch.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

INSERT INTO admin_apis ("group",path,method,description,sort_order,status,created_at,updated_at)
VALUES
 ('运维监控','/admin/ops/incidents','GET','Incident Control workspace',58,'active',NOW(),NOW()),
 ('运维监控','/admin/ops/incidents/settings','PUT','Update Incident Control settings',59,'active',NOW(),NOW()),
 ('运维监控','/admin/ops/incidents/candidates/:id/confirm','POST','Confirm Incident Candidate',60,'active',NOW(),NOW()),
 ('运维监控','/admin/ops/incidents/candidates/:id/dismiss','POST','Dismiss Incident Candidate',61,'active',NOW(),NOW()),
 ('运维监控','/admin/ops/incidents/:id/transition','POST','Transition Incident phase',62,'active',NOW(),NOW()),
 ('运维监控','/admin/ops/incidents/:id/updates','POST','Add internal Incident update',63,'active',NOW(),NOW()),
 ('运维监控','/admin/ops/incidents/:id/public-updates','POST','Publish sanitized Incident update',64,'active',NOW(),NOW()),
 ('运维监控','/admin/ops/incidents/:id/evidence-gap/acknowledge','POST','Acknowledge Incident evidence gap',65,'active',NOW(),NOW())
ON CONFLICT(method,path) DO UPDATE SET
 "group"=EXCLUDED."group",description=EXCLUDED.description,sort_order=EXCLUDED.sort_order,status='active',updated_at=NOW();

INSERT INTO admin_role_apis(role_id,api_id,created_at)
SELECT rm.role_id,api.id,NOW()
FROM admin_role_menus rm
JOIN admin_menus menu ON menu.id=rm.menu_id AND menu.permission_key='admin:ops'
JOIN admin_apis api ON (api.method,api.path) IN (
 ('GET','/admin/ops/incidents'),
 ('PUT','/admin/ops/incidents/settings'),
 ('POST','/admin/ops/incidents/candidates/:id/confirm'),
 ('POST','/admin/ops/incidents/candidates/:id/dismiss'),
 ('POST','/admin/ops/incidents/:id/transition'),
 ('POST','/admin/ops/incidents/:id/updates'),
 ('POST','/admin/ops/incidents/:id/public-updates'),
 ('POST','/admin/ops/incidents/:id/evidence-gap/acknowledge')
)
ON CONFLICT(role_id,api_id) DO NOTHING;
