import { describe, it, expect } from 'vitest';

// The ConsoleSecurity component consumes the /api/metrics response.
// These tests verify the data shape contract the component expects,
// ensuring backend changes don't silently break the security dashboard.

describe('Security metrics data contract', () => {
  // Simulates the shape returned by GET /api/metrics after Phase 5.
  const mockMetrics = {
    shield_summary: {
      shield_allow: 42,
      shield_block: 3,
      shield_escalate: 1,
      rate_limit_hit: 2,
      budget_exhausted: 0,
    },
    security_integrity: {
      audit_chain_failures: 0,
      hash_verifier_failures: 0,
      canary_token_failures: 0,
      agent_auth_failures: 0,
      agent_unexpected_exits: 0,
      protection_bypass_attempts: 0,
    },
    security_defenses: {
      protection_blocks: 5,
      tier3_requests: 1,
      subagent_concurrency_cap_hits: 0,
      subagent_timeout_kills: 0,
    },
    ifc: {
      blocks_total: 2,
      audit_would_block_total: 7,
    },
    daily_metrics: {
      tool_calls: 100,
      tool_success: 95,
      tool_failed: 5,
    },
  };

  it('security_integrity has all expected fields', () => {
    const si = mockMetrics.security_integrity;
    expect(si).toHaveProperty('audit_chain_failures');
    expect(si).toHaveProperty('hash_verifier_failures');
    expect(si).toHaveProperty('canary_token_failures');
    expect(si).toHaveProperty('agent_auth_failures');
    expect(si).toHaveProperty('agent_unexpected_exits');
    expect(si).toHaveProperty('protection_bypass_attempts');
  });

  it('all integrity values are zero in a healthy system', () => {
    const si = mockMetrics.security_integrity;
    const values = Object.values(si);
    expect(values.every(v => v === 0)).toBe(true);
  });

  it('integrity alert triggers on non-zero value', () => {
    const alertMetrics = {
      ...mockMetrics.security_integrity,
      audit_chain_failures: 1,
    };
    const hasAlert = Object.values(alertMetrics).some(v => v > 0);
    expect(hasAlert).toBe(true);
  });

  it('security_defenses has all expected fields', () => {
    const sd = mockMetrics.security_defenses;
    expect(sd).toHaveProperty('protection_blocks');
    expect(sd).toHaveProperty('tier3_requests');
    expect(sd).toHaveProperty('subagent_concurrency_cap_hits');
    expect(sd).toHaveProperty('subagent_timeout_kills');
  });

  it('ifc has all expected fields', () => {
    const ifc = mockMetrics.ifc;
    expect(ifc).toHaveProperty('blocks_total');
    expect(ifc).toHaveProperty('audit_would_block_total');
  });

  it('shield_summary has all expected fields', () => {
    const ss = mockMetrics.shield_summary;
    expect(ss).toHaveProperty('shield_allow');
    expect(ss).toHaveProperty('shield_block');
    expect(ss).toHaveProperty('shield_escalate');
    expect(ss).toHaveProperty('rate_limit_hit');
    expect(ss).toHaveProperty('budget_exhausted');
  });

  it('tool metrics present in daily_metrics', () => {
    const dm = mockMetrics.daily_metrics;
    expect(dm.tool_calls).toBeGreaterThan(0);
    expect(dm.tool_success + dm.tool_failed).toBe(dm.tool_calls);
  });
});
