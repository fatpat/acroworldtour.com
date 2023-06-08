# Acro World Tour - Next.js Frontend

## Getting Started

First, run the development server:

`pnpm dev`

- Server runs on [http://localhost:3000](http://localhost:3000)
- [Next.js Documentation](https://nextjs.org/docs)
- [Tailwind CSS Documentation](https://tailwindcss.com/docs)
- [AWT API Documentation](https://api.acroworldtour.com/docs)

## Must Know

- Use the /public API endpoints.
- We are using next's [page router](https://nextjs.org/docs/pages) (not the app router).
- The website is static, and the data fetching happens on the client, using [SWR](https://swr.vercel.app/).
- Check the [tailwind config file](./tailwind.config.js) for the custom 'awt-dark' colour alias used throughout the project. This permits a global colour change from one place.
- If the API changes, please run `pnpm interface` tu update the typescript interfaces.
