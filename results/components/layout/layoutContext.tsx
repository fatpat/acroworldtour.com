/* eslint-disable no-unused-vars */
import { createContext, useContext } from "react";

interface ILayoutContext {
  pageTitle: string;
  pageDescription: string;
  headerTitle: string;
  headerSubtitle: string;
  activeNav: string;
  setPageTitle: (value: string) => void;
  setPageDescription: (value: string) => void;
  setHeaderTitle: (value: string) => void;
  setHeaderSubtitle: (value: string) => void;
  setActiveNav: (value: string) => void;
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
