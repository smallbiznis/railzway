import { useEffect, useState } from "react"
import { useOrgStore } from "@/stores/orgStore"
import { admin } from "@/api/client"
import { useAuthStore } from "@/stores/authStore"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"

interface Member {
  user_id: string
  display_name: string
  email: string
  role: string
  avatar: string
  created_at: string
}

export function TeamMembersList() {
  const currentOrg = useOrgStore((s) => s.currentOrg)
  const currentOrgId = currentOrg?.id

  // const params = useParams()
  // const currentOrgId = params.id

  const currentUser = useAuthStore((s) => s.user)
  const [members, setMembers] = useState<Member[]>([])
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    if (!currentOrgId) return

    setIsLoading(true)
    admin
      .get(`/organizations/${currentOrgId}/members`)
      .then((res) => {
        setMembers(res.data?.data ?? [])
      })
      .catch((err) => {
        console.error("Failed to load members", err)
      })
      .finally(() => {
        setIsLoading(false)
      })
  }, [currentOrgId])

  if (isLoading) {
    return <div className="text-sm text-text-muted">Loading team members...</div>
  }

  if (members.length === 0) {
    return (
      <div className="text-sm text-text-muted">No team members found.</div>
    )
  }

  return (
    <div className="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>User</TableHead>
            <TableHead>Role</TableHead>
            <TableHead>Joined</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {members.map((member) => {
            const isMe = member.user_id === currentUser?.id
            return (
              <TableRow key={member.user_id} className={isMe ? "bg-bg-subtle/50" : ""}>
                <TableCell>
                  <div className="flex items-center gap-3">
                    <Avatar className="h-8 w-8 rounded-lg">
                      <AvatarImage src={member.avatar} />
                      <AvatarFallback className="rounded-lg">
                        {member.display_name?.slice(0, 2).toUpperCase() || "CN"}
                      </AvatarFallback>
                    </Avatar>
                    <div className="flex flex-col">
                      <span className="font-medium text-sm flex items-center gap-2">
                        {member.display_name}
                        {isMe && <Badge variant="secondary" className="text-[10px] h-4 px-1">YOU</Badge>}
                      </span>
                      <span className="text-xs text-text-muted">{member.email}</span>
                    </div>
                  </div>
                </TableCell>
                <TableCell>
                  <Badge variant="outline" className="capitalize">{member.role.toLowerCase()}</Badge>
                </TableCell>
                <TableCell className="text-text-muted text-sm">
                  {new Date(member.created_at).toLocaleDateString()}
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}
