import { useState } from "react"
import { Link, useNavigate, useParams } from "react-router-dom"

import { admin } from "@/api/client"
import { ForbiddenState } from "@/components/forbidden-state"
import { Alert } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { getErrorMessage } from "@/lib/api-errors"
import { canManageBilling } from "@/lib/roles"
import { useOrgStore } from "@/stores/orgStore"

const aggregationOptions = [
  { label: "Sum", value: "SUM" },
  { label: "Count", value: "COUNT" },
  { label: "Max", value: "MAX" },
  { label: "Min", value: "MIN" },
  { label: "Average", value: "AVG" },
]

export default function OrgMeterCreatePage() {
  const { orgId } = useParams()
  const navigate = useNavigate()
  const role = useOrgStore((state) => state.currentOrg?.role)
  const canManage = canManageBilling(role)

  const [name, setName] = useState("")
  const [code, setCode] = useState("")
  const [aggregation, setAggregation] = useState("SUM")
  const [unit, setUnit] = useState("unit")
  const [description, setDescription] = useState("")
  const [active, setActive] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!canManage) {
      setError("You do not have permission to create meters.")
      return
    }
    if (!orgId) {
      setError("Organization is missing.")
      return
    }

    setError(null)
    setIsSubmitting(true)

    try {
      const payload = {
        organization_id: orgId,
        name: name.trim(),
        code: code.trim(),
        aggregation_type: aggregation,
        unit: unit.trim(),
        active,
        description: description.trim() || undefined,
      }
      const res = await admin.post("/meters", payload)
      const meterId = res.data?.data?.id
      if (meterId) {
        navigate(`/orgs/${orgId}/meter/${meterId}`, { replace: true })
      } else {
        navigate(`/orgs/${orgId}/meter`, { replace: true })
      }
    } catch (err: any) {
      setError(getErrorMessage(err, "Unable to create meter."))
    } finally {
      setIsSubmitting(false)
    }
  }

  if (!canManage) {
    return <ForbiddenState description="You do not have access to create meters." />
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-2">
        <div className="text-sm text-text-muted">
          <Link className="text-accent-primary hover:underline" to={`/orgs/${orgId}/meter`}>
            Meters
          </Link>{" "}
          / Create meter
        </div>
        <h1 className="text-2xl font-semibold">Create meter</h1>
      </div>

      <form className="grid gap-6 lg:grid-cols-[2fr_1fr]" onSubmit={handleSubmit}>
        <Card>
          <CardHeader>
            <CardTitle>Meter configuration</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {error && <Alert variant="destructive">{error}</Alert>}
            <div className="space-y-2">
              <Label htmlFor="meter-name">Meter name</Label>
              <Input
                id="meter-name"
                data-testid="meter-name"
                placeholder="API requests"
                value={name}
                onChange={(event) => setName(event.target.value)}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="meter-code">Event name</Label>
              <Input
                id="meter-code"
                data-testid="meter-code"
                placeholder="api_requests"
                value={code}
                onChange={(event) => setCode(event.target.value)}
                required
              />
            </div>
            <div className="space-y-2">
              <Label>Aggregation method</Label>
              <Select value={aggregation} onValueChange={setAggregation}>
                <SelectTrigger data-testid="meter-aggregation">
                  <SelectValue placeholder="Select aggregation method" />
                </SelectTrigger>
                <SelectContent>
                  {aggregationOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="meter-unit">Unit</Label>
              <Input
                id="meter-unit"
                data-testid="meter-unit"
                placeholder="unit"
                value={unit}
                onChange={(event) => setUnit(event.target.value)}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="meter-description">Description</Label>
              <Textarea
                id="meter-description"
                data-testid="meter-description"
                placeholder="Optional description for internal teams."
                rows={3}
                value={description}
                onChange={(event) => setDescription(event.target.value)}
              />
            </div>
            <div className="flex items-center justify-between rounded-lg border p-3">
              <div className="space-y-1">
                <Label htmlFor="meter-active">Active</Label>
                <p className="text-text-muted text-xs">
                  Disable to stop ingesting usage events.
                </p>
              </div>
              <Switch
                id="meter-active"
                data-testid="meter-active"
                checked={active}
                onCheckedChange={setActive}
              />
            </div>
            <div className="flex flex-wrap items-center gap-3">
              <Button type="submit" data-testid="meter-submit" disabled={isSubmitting}>
                {isSubmitting ? "Creating..." : "Create meter"}
              </Button>
              <Button variant="outline" asChild>
                <Link to={`/orgs/${orgId}/meter`}>Cancel</Link>
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Preview</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm text-text-muted">
            <div>
              <div className="font-medium text-text-primary">Aggregate by {aggregation.toLowerCase()}</div>
              <p>Usage events will be aggregated into a single value.</p>
            </div>
            <div className="rounded-lg border border-dashed p-4 text-center">
              <div className="text-xs uppercase tracking-wide">Sample usage</div>
              <div className="mt-2 text-2xl font-semibold text-text-primary">16</div>
              <div className="text-xs">end of cycle value</div>
            </div>
          </CardContent>
        </Card>
      </form>
    </div>
  )
}
