const path = require('path')

module.exports = {
  basePath: `${process.env.NEXT_PUBLIC_APP_BASE_PATH}`,
  trailingSlash: false,
  reactStrictMode: false,
  images: {
    loader: "custom",
    unoptimized: true
  },
  experimental: {
    esmExternals: false,
    jsconfigPaths: true // enables it for both jsconfig.json and tsconfig.json
  },
  webpack: config => {
    config.resolve.alias = {
      ...config.resolve.alias,
      apexcharts: path.resolve(__dirname, './node_modules/apexcharts-clevision')
    }

    return config
  }
}
