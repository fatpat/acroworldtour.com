const path = require('path')

const nextConfig = {
  output: 'export',

  basePath: `${process.env.NEXT_PUBLIC_APP_BASE_PATH}`,
  trailingSlash: false,
  reactStrictMode: false,
  images: {
    loader: "custom",
    unoptimized: true
  },
  experimental: {
    esmExternals: false,
  },
  webpack: config => {
    config.resolve.alias = {
      ...config.resolve.alias,
      apexcharts: path.resolve(__dirname, './node_modules/apexcharts-clevision')
    }

    return config
  }
}

module.exports = nextConfig
