-- Migration 007: Assignments and referrals
-- Assignments track which responder owns a case.
-- Acceptance is a separate, explicit action — assignment != acceptance.
-- version tracks optimistic locking to prevent concurrent acceptance races.

CREATE TABLE assignments (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    case_id             UUID NOT NULL REFERENCES cases(id),
    responder_id        UUID NOT NULL REFERENCES staff_members(id),
    organization_id     UUID NOT NULL REFERENCES organizations(id),
    assigned_by         UUID NOT NULL REFERENCES staff_members(id),
    assigned_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- accepted_at is NULL until the responder explicitly accepts.
    -- assignment alone is NOT acceptance.
    accepted_at         TIMESTAMPTZ,
    -- escalation_due_at: the worker fires an escalation if not accepted by this time.
    escalation_due_at   TIMESTAMPTZ,
    -- unassigned_at: when reassigned, old assignment is closed here.
    unassigned_at       TIMESTAMPTZ,
    -- version is incremented on accept/reassign to detect concurrent mutations.
    version             INTEGER NOT NULL DEFAULT 1
);

COMMENT ON TABLE assignments IS
    'Case assignments. accepted_at is only set by explicit responder action — assignment alone is not acceptance.';
COMMENT ON COLUMN assignments.accepted_at IS
    'NULL until responder explicitly calls accept. Being assigned is never the same as accepting responsibility.';

CREATE INDEX assignments_case_id_idx ON assignments(case_id);
CREATE INDEX assignments_responder_id_idx ON assignments(responder_id);
CREATE INDEX assignments_escalation_due_idx ON assignments(escalation_due_at)
    WHERE escalation_due_at IS NOT NULL AND accepted_at IS NULL AND unassigned_at IS NULL;

-- Referrals: a handoff to another organization (must be explicitly acknowledged).
-- An unanswered export or email is NOT a completed referral.

CREATE TABLE referrals (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    case_id             UUID NOT NULL REFERENCES cases(id),
    from_organization   UUID NOT NULL REFERENCES organizations(id),
    to_organization     UUID NOT NULL REFERENCES organizations(id),
    referred_by         UUID NOT NULL REFERENCES staff_members(id),
    referred_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- acknowledged_at is set only when the receiving org explicitly accepts.
    acknowledged_at     TIMESTAMPTZ,
    acknowledged_by     UUID REFERENCES staff_members(id),
    notes               TEXT
);

COMMENT ON TABLE referrals IS
    'Case referrals to other organizations. acknowledged_at is only set by explicit receiving-org acceptance. Unacknowledged referrals are not completed handoffs.';

CREATE INDEX referrals_case_id_idx ON referrals(case_id);
CREATE INDEX referrals_to_org_idx ON referrals(to_organization, acknowledged_at)
    WHERE acknowledged_at IS NULL;
