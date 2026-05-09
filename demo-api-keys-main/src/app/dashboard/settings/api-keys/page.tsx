import { APIKeys } from '@clerk/nextjs'
import { RequestTester } from '@/components/request-tester'

export default function SettingsPage() {
  return (
    <div className="flex flex-col gap-4 p-8 pt-6">
      <h1 className="font-semibold text-lg">API keys</h1>
      <APIKeys showDescription />
      <RequestTester />
    </div>
  )
}
