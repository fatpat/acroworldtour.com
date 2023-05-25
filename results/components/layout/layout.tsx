import Head from "next/head";
import { useState } from "react";

import Header from "./header";
import LayoutContext from "./layoutContext";
import Nav from "./nav";

interface LayoutProps {
  children: React.ReactNode;
  fontClass: string;
}

const Layout = ({ children, fontClass }: LayoutProps) => {
  const [pageTitle, setPageTitle] = useState("");
  const [pageDescription, setPageDescription] = useState("");
  const [headerTitle, setHeaderTitle] = useState("");
  const [headerSubtitle, setHeaderSubtitle] = useState("");
  const [activeNav, setActiveNav] = useState("");

  return (
    <LayoutContext.Provider
      value={{
        pageTitle,
        pageDescription,
        headerTitle,
        headerSubtitle,
        activeNav,
        setPageTitle,
        setPageDescription,
        setHeaderTitle,
        setHeaderSubtitle,
        setActiveNav,
      }}
    >
      <Head>
        <title>{pageTitle}</title>
        <meta name="description" content={pageDescription} />
      </Head>
      <Header
        fontClass={fontClass}
        headerTitle={headerTitle}
        headerSubtitle={headerSubtitle}
      />
      {children}
      <Nav fontClass={fontClass} activeNav={activeNav} />
    </LayoutContext.Provider>
  );
};

export default Layout;
