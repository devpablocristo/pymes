import { Header } from '@/components/header'

export default function DefaultLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <div className="flex min-h-svh flex-col">
      <Header />
      <main className="flex flex-1 items-center justify-center">
        {children}
      </main>
    </div>
  )
}
