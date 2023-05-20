import "@/styles/globals.css";
import "semantic-ui-flag/flag.min.css";

import classNames from "classnames";
import type { AppProps } from "next/app";
import { Montserrat } from "next/font/google";

import Layout from "@/components/layout/layout";

const montserrat = Montserrat({
  subsets: ["latin"],
});

const App = ({ Component, pageProps }: AppProps) => {
  return (
    <Layout>
      <main
        className={classNames(
          montserrat.className,
          "flex w-full flex-1 flex-col items-center pb-8",
          "lg:pl-52 lg:pt-24"
        )}
      >
        <Component {...pageProps} />
      </main>
    </Layout>
  );
};

export default App;
