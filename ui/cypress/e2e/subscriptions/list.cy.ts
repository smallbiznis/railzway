const formatStatus = (value?: string) => {
  if (!value) return "-"
  switch (value.toUpperCase()) {
    case "ACTIVE":
      return "Active"
    case "PAUSED":
      return "Paused"
    case "CANCELED":
      return "Canceled"
    case "ENDED":
      return "Ended"
    case "DRAFT":
      return "Draft"
    default:
      return value
  }
}

describe("Subscriptions list", () => {
  const orgId = Cypress.env("E2E_ORG_ID") || "2002990275537932288"
  const subscriptionsPath = `/orgs/${orgId}/subscriptions`

  beforeEach(() => {
    cy.loginAsAdmin({ orgId })
  })

  it("renders subscriptions list and status badges", () => {
    cy.intercept("GET", "/api/subscriptions*").as("getSubscriptions")

    cy.visit(subscriptionsPath)

    cy.findByRole("heading", { name: /^Subscriptions$/, level: 1 }).should(
      "be.visible"
    )
    cy.findAllByRole("link", { name: /create subscription/i })
      .first()
      .should("be.visible")

    cy.wait("@getSubscriptions").then((interception) => {
      expect(interception.response?.statusCode).to.eq(200)
      const subscriptions = interception.response?.body?.data ?? []
      expect(subscriptions).to.be.an("array")

      if (subscriptions.length > 0) {
        cy.findByRole("table").within(() => {
          cy.findAllByRole("row").should("have.length.greaterThan", 1)
        })
        const rawStatus =
          subscriptions[0]?.status || subscriptions[0]?.Status || "-"
        const statusLabel = formatStatus(String(rawStatus))
        if (statusLabel && statusLabel !== "-") {
          cy.findByText(statusLabel).should("be.visible")
        }
      } else {
        cy.findByText(/no subscriptions yet/i).should("be.visible")
      }
    })
  })
})
