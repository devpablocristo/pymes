import { Bot } from 'lucide-react'
import { SidebarMenuButton } from '@/components/ui/sidebar'

export function ActiveOrg() {
  return (
    <SidebarMenuButton className="pl-0" size="lg">
      <Bot className="size-7!" />
      <div className="grid flex-1 text-left text-sm leading-tight">
        <span className="truncate font-medium">AgentOps</span>
      </div>
    </SidebarMenuButton>
  )
}
