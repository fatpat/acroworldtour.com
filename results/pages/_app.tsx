import "@/styles/globals.css";
import "semantic-ui-flag/flag.min.css";

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

export default function App({ Component, pageProps }: CustomAppProps) {
  return (
    <Layout
      pageTitle={Component.pageTitle}
      pageDescription={Component.pageDescription}
      headerTitle={Component.headerTitle}
      headerSubtitle={Component.headerSubtitle}
    >
      <main className={montserrat.className}>
        <Component {...pageProps} />
      </main>
    </Layout>
  );
}
