import "@/styles/globals.css";
import "semantic-ui-flag/flag.min.css";

import classNames from "classnames";
import type { AppProps } from "next/app";
import { Montserrat } from "next/font/google";
import { NextComponentType } from "next/types";

import Layout from "@/components/layout";

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
          "mb-28 flex max-w-md flex-col px-4 gap-6",
          "lg:mb-8 lg:ml-52 lg:mt-28 lg:max-w-full lg:justify-evenly lg:gap-4"
        )}
      >
        <Component {...pageProps} />
      </main>
    </Layout>
  );
};

export default App;
