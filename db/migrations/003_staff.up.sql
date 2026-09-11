-- Migration 003: Staff members, roles, and grants
-- Staff belong to organizations. Roles are explicit grants, not implied by org membership.
-- Available roles: responder, supervisor, alert_preparer, alert_approver, admin

CREATE TABLE staff_members (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    username        VARCHAR(100) NOT NULL UNIQUE,
    -- password_hash uses bcrypt. Never store plaintext.
    password_hash   VARCHAR(255) NOT NULL,
    display_name    VARCHAR(255) NOT NULL,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE staff_members IS
    'Staff accounts. Auth uses local JWT stub for demo; replace with OIDC in production.';

CREATE TABLE role_grants (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    staff_id        UUID NOT NULL REFERENCES staff_members(id),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    role            VARCHAR(50) NOT NULL CHECK (role IN ('responder', 'supervisor', 'alert_preparer', 'alert_approver', 'admin')),
    granted_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    granted_by      UUID REFERENCES staff_members(id),
    revoked_at      TIMESTAMPTZ,
    UNIQUE (staff_id, organization_id, role)
);

COMMENT ON TABLE role_grants IS
    'Explicit role grants. A staff member has no permissions without a role grant. Revocation ends the grant.';

CREATE INDEX role_grants_staff_id_idx ON role_grants(staff_id, organization_id);

-- FICTIONAL demo staff members
-- Passwords are 'demo-password' hashed with bcrypt cost 12.
-- These are synthetic accounts for testing only.
-- bcrypt hash of 'demo-password': $2a$12$LQv3c1yqBwQiXJaopFVV3u.WtCRjUBJHOUFJLVKLDWu.w6H4qhBdq
INSERT INTO staff_members (id, organization_id, username, password_hash, display_name) VALUES
    ('00000000-0000-0000-0000-000000000020',
     '00000000-0000-0000-0000-000000000001',
     'responder.demo',
     '$2a$10$13mHfJLprkEY2PTVXVSHeuPIJewbnBpb.nSjb.r6uiOqAKMcO91bu',
     '[FICTIONAL] Demo Responder'),
    ('00000000-0000-0000-0000-000000000021',
     '00000000-0000-0000-0000-000000000001',
     'supervisor.demo',
     '$2a$10$13mHfJLprkEY2PTVXVSHeuPIJewbnBpb.nSjb.r6uiOqAKMcO91bu',
     '[FICTIONAL] Demo Supervisor'),
    ('00000000-0000-0000-0000-000000000022',
     '00000000-0000-0000-0000-000000000001',
     'preparer.demo',
     '$2a$10$13mHfJLprkEY2PTVXVSHeuPIJewbnBpb.nSjb.r6uiOqAKMcO91bu',
     '[FICTIONAL] Demo Alert Preparer'),
    ('00000000-0000-0000-0000-000000000023',
     '00000000-0000-0000-0000-000000000001',
     'approver.demo',
     '$2a$10$13mHfJLprkEY2PTVXVSHeuPIJewbnBpb.nSjb.r6uiOqAKMcO91bu',
     '[FICTIONAL] Demo Alert Approver');

INSERT INTO role_grants (staff_id, organization_id, role) VALUES
    ('00000000-0000-0000-0000-000000000020', '00000000-0000-0000-0000-000000000001', 'responder'),
    ('00000000-0000-0000-0000-000000000021', '00000000-0000-0000-0000-000000000001', 'responder'),
    ('00000000-0000-0000-0000-000000000021', '00000000-0000-0000-0000-000000000001', 'supervisor'),
    ('00000000-0000-0000-0000-000000000022', '00000000-0000-0000-0000-000000000001', 'alert_preparer'),
    ('00000000-0000-0000-0000-000000000023', '00000000-0000-0000-0000-000000000001', 'alert_approver');
