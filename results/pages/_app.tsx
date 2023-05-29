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
          "flex min-h-screen w-full flex-col items-center overflow-y-scroll px-2 pb-8 pt-4",
          "lg:h-screen lg:pb-4 lg:pl-52 lg:pr-4 lg:pt-28"
        )}
      >
        <Component {...pageProps} />
      </main>
    </Layout>
  );
};

export default App;
