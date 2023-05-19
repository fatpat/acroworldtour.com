import "@/styles/globals.css";
import "semantic-ui-flag/flag.min.css";

import classNames from "classnames";
import type { AppProps } from "next/app";
import { Montserrat } from "next/font/google";
import { NextComponentType } from "next/types";

import Layout from "@/components/layout/layout";

const montserrat = Montserrat({
  subsets: ["latin"],
});

type CustomAppProps<P = {}> = AppProps<P> & {
  Component: NextComponentType & {
    pageTitle: string;
    pageDescription: string;
    headerTitle?: string;
    headerSubtitle?: string;
  };
};

const App = ({ Component, pageProps }: CustomAppProps) => {
  return (
    <Layout
      pageTitle={Component.pageTitle}
      pageDescription={Component.pageDescription}
      headerTitle={Component.headerTitle}
      headerSubtitle={Component.headerSubtitle}
    >
      <main
        className={classNames(
          montserrat.className,
          "mb-8 flex w-full flex-1 flex-col items-center prose max-w-max",
          "lg:mb-8 lg:ml-52 lg:mt-24"
        )}
      >
        <Component {...pageProps} />
      </main>
    </Layout>
  );
};

export default App;
