import Head from "next/head";

import Header from "./header";
import Nav from "./nav";

interface LayoutProps {
  children: React.ReactNode;
  pageTitle: string;
  pageDescription?: string;
  headerTitle?: string;
  headerSubtitle?: string;
}

const Layout = ({
  children,
  pageTitle,
  pageDescription,
  headerTitle,
  headerSubtitle,
}: LayoutProps) => (
  <>
    <Head>
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />
    </Head>
    <Header headerTitle={headerTitle} headerSubtitle={headerSubtitle} />
    <Nav pageTitle={pageTitle} />
    {children}
  </>
);

export default Layout;
