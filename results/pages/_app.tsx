import "@/styles/globals.css";
import "semantic-ui-flag/flag.min.css";

import classNames from "classnames";
import type { AppProps } from "next/app";
import { Montserrat } from "next/font/google";

import Layout from "@/components/layout/layout";

const font = Montserrat({
  subsets: ["latin"],
});

const App = ({ Component, pageProps }: AppProps) => {
  return (
    <Layout fontClass={font.className}>
      <main
        className={classNames(
          font.className,
          "flex w-full flex-col items-center overflow-y-scroll pb-40 pt-4 sm:px-2",
          "lg:pb-4 lg:pl-52 lg:pr-4 lg:pt-28"
        )}
      >
        <Component {...pageProps} />
      </main>
    </Layout>
  );
};

export default App;
