import { createContext, useContext, useState, type ReactNode } from 'react'
import { createBrowserRouter, RouterProvider } from 'react-router-dom'

const RouteContent = createContext<ReactNode>(null)
function SessionRoute() { return useContext(RouteContent) }
let sessionRouter: ReturnType<typeof createBrowserRouter> | undefined

// Keep the data router stable across access-token refreshes. Session props flow
// through context, so refreshing authentication never remounts an input form.
export function SessionRouter({ children }: { children: ReactNode }) {
  const [router] = useState(() => sessionRouter ??= createBrowserRouter([{ path: '*', element: <SessionRoute /> }]))
  return <RouteContent.Provider value={children}><RouterProvider router={router} /></RouteContent.Provider>
}
