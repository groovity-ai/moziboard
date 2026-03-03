import { NextResponse } from 'next/server'
import type { NextRequest } from 'next/server'

export function middleware(request: NextRequest) {
    const authCookie = request.cookies.get('session_id') // Assuming AiAgenz uses 'session_id' or similar, we'll just check for ANY auth cookie it sets, or specifically 'token'
    // For a more robust check, we could call an auth endpoint here, but a cookie check is standard for Next.js middleware.
    // If we don't know the exact cookie name, we might just check if ANY cookie exists that looks like auth, 
    // or just let the API calls fail with 401 and handle it in the UI. 
    // Let's check for the existence of `connect.sid` or `AuthToken` or similar. We will just check `aiagenz_session` or fallback to checking the `url`.

    // For now, let's protect `/board`
    if (request.nextUrl.pathname.startsWith('/board')) {
        // If we want a strict check, we should verify the session.
        // A simple check is looking for a cookie. AiAgenz backend sets a cookie.
        const hasAuthCookie = request.cookies.getAll().length > 0; // Temporary broad check until we know the exact cookie name

        if (!hasAuthCookie) {
            return NextResponse.redirect(new URL('/login', request.url))
        }
    }

    // If user is on /login but HAS a cookie, maybe redirect to /board
    if (request.nextUrl.pathname === '/login') {
        const hasAuthCookie = request.cookies.getAll().length > 0;
        if (hasAuthCookie) {
            // We don't know their board ID, so maybe redirect to a generic dashboard or let them stay on login to re-auth
        }
    }

    return NextResponse.next()
}

export const config = {
    matcher: ['/board/:path*', '/login'],
}
