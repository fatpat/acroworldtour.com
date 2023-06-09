import { createContext, useContext } from "react";

interface ILayoutContext {
  pageTitle: string;
  pageDescription: string;
  headerTitle: string;
  headerSubtitle: string;
  activeNav: string;
  setPageTitle: Function;
  setPageDescription: Function;
  setHeaderTitle: Function;
  setHeaderSubtitle: Function;
  setActiveNav: Function;
}

const LayoutContext = createContext<ILayoutContext>({
  pageTitle: "",
  pageDescription: "",
  headerTitle: "",
  headerSubtitle: "",
  activeNav: "",
  setPageTitle: () => {},
  setPageDescription: () => {},
  setHeaderTitle: () => {},
  setHeaderSubtitle: () => {},
  setActiveNav: () => {},
});

export const useLayout = () => useContext(LayoutContext);

export default LayoutContext;
