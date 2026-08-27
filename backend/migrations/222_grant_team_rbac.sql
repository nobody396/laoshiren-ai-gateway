SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

INSERT INTO admin_menus(name,name_en,type,path,component,icon,permission_key,sort_order,status)
VALUES ('团队管理','Teams','menu','/admin/teams','','users','admin:teams',25,'active')
ON CONFLICT(permission_key) DO UPDATE SET
 name=EXCLUDED.name,name_en=EXCLUDED.name_en,type=EXCLUDED.type,path=EXCLUDED.path,
 component=EXCLUDED.component,icon=EXCLUDED.icon,sort_order=EXCLUDED.sort_order,
 status='active',updated_at=NOW();

INSERT INTO admin_apis("group",path,method,description,sort_order,status,created_at,updated_at) VALUES
 ('团队管理','/admin/teams','GET','List teams',1,'active',NOW(),NOW()),
 ('团队管理','/admin/teams','POST','Create team',2,'active',NOW(),NOW()),
 ('团队管理','/admin/teams/:id','GET','Read team',3,'active',NOW(),NOW()),
 ('团队管理','/admin/teams/:id','PATCH','Update team',4,'active',NOW(),NOW()),
 ('团队管理','/admin/teams/:id','DELETE','Dissolve team',5,'active',NOW(),NOW()),
 ('团队管理','/admin/teams/:id/members','GET','List team members',6,'active',NOW(),NOW()),
 ('团队管理','/admin/teams/:id/usage','GET','Read team usage',7,'active',NOW(),NOW()),
 ('团队管理','/admin/teams/:id/force-transfer','POST','Force team ownership transfer',8,'active',NOW(),NOW())
ON CONFLICT(method,path) DO UPDATE SET
 "group"=EXCLUDED."group",description=EXCLUDED.description,sort_order=EXCLUDED.sort_order,
 status='active',updated_at=NOW();

INSERT INTO admin_role_menus(role_id,menu_id,created_at)
SELECT role.id,menu.id,NOW()
FROM admin_roles role
JOIN admin_menus menu ON menu.permission_key='admin:teams'
WHERE role.is_super_admin=TRUE AND role.status='active'
ON CONFLICT(role_id,menu_id) DO NOTHING;

INSERT INTO admin_role_apis(role_id,api_id,created_at)
SELECT role.id,api.id,NOW()
FROM admin_roles role
JOIN admin_apis api ON api."group"='团队管理'
WHERE role.is_super_admin=TRUE AND role.status='active'
ON CONFLICT(role_id,api_id) DO NOTHING;
