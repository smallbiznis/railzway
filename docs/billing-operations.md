# Billing Operations and Collections

Railzway isolates "Billing Operations" into a dedicated domain to handle the human-in-the-loop aspects of revenue recovery.

While Railzway automates billing cycles and invoice generation, **Dunning (Collections)** is often a stateful, human-driven process for enterprise B2B.

---

## Philosophy

Railzway treats collections as **Assignment-based Work**.

- **System Responsibility:** Identify past-due invoices, assess risk, and assign work items.
- **Human Responsibility:** Contact customer, negotiate terms, and record outcomes.

We do not implement "automated email sequences" inside the billing engine. These are better handled by external CRM or marketing automation tools triggered via webhooks.

---

## Assignments

When an invoice becomes past due or meets a risk threshold, Railzway creates a **Billing Operation Assignment**.

### Lifecycle

1.  **Open**: Created automatically or manually.
2.  **Assigned**: Linked to a specific billing agent/user.
3.  **In Progress**: Follow-ups are being conducted.
4.  **Resolved**: Invoice paid or written off.
5.  **Closed**: Operation complete.

---

## Follow-Up Tracking

Railzway provides a primitive for tracking manual follow-ups:

- **Start Manual Email**: Agents trigger a "Record Follow Up" action.
- **Client Deployment**: The UI opens the agent's local email client (`mailto:`).
- **Audit**: The system records that a follow-up was initiated, increments the `follow_up_count`, and updates `last_follow_up_at`.

This ensures that while the *communication* happens externally (Gmail, Outlook), the *cadence and effort* are tracked immutably within Railzway.
