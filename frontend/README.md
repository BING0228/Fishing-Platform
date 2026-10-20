# Spearhook Frontend (React)

International stack:

- **React 19** + **TypeScript**
- **Next.js** (App Router) with **`output: 'export'`** for static files embedded by Go
- **React Router** for in-app SPA routes (deep links work via Go `NoRoute` → `index.html`)
- **Tailwind CSS** + **shadcn/ui**-style Radix components
- **axios** / **zustand** / lightweight i18n (English source + zh-CN, zh-TW, ja, ko)

## Develop

```bash
# Backend on :8000 (CORS already open)
cd frontend
npm install
npm run dev   # http://127.0.0.1:8081  (API via NEXT_PUBLIC_API_BASE)
```

## Production build (Go embed)

```bash
npm run build   # writes frontend/dist (used by //go:embed frontend/dist)
```

`build-all.sh` still runs `npm run build` then embeds `frontend/dist`.
