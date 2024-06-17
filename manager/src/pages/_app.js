// ** Next Imports
import Head from 'next/head'
import { Router } from 'next/router'

// ** Loader Import
import NProgress from 'nprogress'

// ** Emotion Imports
// ** Config Imports
import themeConfig from 'src/configs/themeConfig'

// ** Component Imports
import UserLayout from 'src/layouts/UserLayout'
import ThemeComponent from 'src/@core/theme/ThemeComponent'

// ** Contexts
import { SettingsConsumer, SettingsProvider } from 'src/@core/context/settingsContext'

// ** React Perfect Scrollbar Style
import 'react-perfect-scrollbar/dist/css/styles.css'

// ** Global css styles
import '../../styles/globals.css'

// ** notistack
import { SnackbarProvider } from 'notistack';

// ** auth
import { UserProvider, RequestError } from "@auth0/nextjs-auth0/client";

// ** local
import { APIRequest } from 'src/util/backend'

// ** jquery
import $ from 'jquery'

//const clientSideEmotionCache = createEmotionCache()

// ** Pace Loader
if (themeConfig.routingLoader) {
  Router.events.on('routeChangeStart', () => {
    NProgress.start()
  })
  Router.events.on('routeChangeError', () => {
    NProgress.done()
  })
  Router.events.on('routeChangeComplete', () => {
    NProgress.done()
  })
}

// ** Configure JSS & ClassName
const App = props => {
  const { Component, pageProps } = props
  const { user } = pageProps

  // Variables
  const getLayout = Component.getLayout ?? (page => <UserLayout>{page}</UserLayout>)

  const authFetcher = async(route) => {
    const [err, body, headers] = await APIRequest(route, {expect_json: true})
    if (err) throw new RequestError(err)
    return body
  }

  if (typeof window !== "undefined") {
    window.onbeforeprint = (event) => {
      $('.hideToPrint').hide()
    }
    window.onafterprint = (event) => {
      $('.hideToPrint').show()
    }
  }

  return (
    <UserProvider user={user} loginUrl="/login" profileUrl="/auth/me" fetcher={authFetcher}>
        <Head>
          <title>{`${themeConfig.templateName} - 2022`}</title>
          <meta
            name='description'
            content={`${themeConfig.templateName} – Material Design React Admin Dashboard Template – is the most developer friendly & highly customizable Admin Dashboard Template based on MUI v5.`}
          />
          <meta name='keywords' content='Material Design, MUI, Admin Template, React Admin Template' />
          <meta name='viewport' content='initial-scale=1, width=device-width' />
        </Head>

        <SettingsProvider>
          <SettingsConsumer>
            {({ settings }) => {
              return (
                <ThemeComponent settings={settings}>
                  <SnackbarProvider
                    anchorOrigin={{
                      vertical: 'top',
                      horizontal: 'center',
                    }}
                    preventDuplicate
                  >
                    {getLayout(<Component {...pageProps} />)}
                  </SnackbarProvider>
                </ThemeComponent>
              )
            }}
          </SettingsConsumer>
        </SettingsProvider>
    </UserProvider>
  )
}

export default App
