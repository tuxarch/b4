import "@fontsource-variable/jetbrains-mono";
import {
  Accordion,
  AppShell,
  Badge,
  CSSVariablesResolver,
  Card,
  NavLink,
  Table,
  ThemeIcon,
  Tooltip,
  createTheme,
} from "@mantine/core";

export const resolver: CSSVariablesResolver = () => ({
  variables: {},
  light: {},
  dark: {
    "--mantine-color-body": "#0c0a09",
  },
});

export const theme = createTheme({
  fontFamily: "JetBrains Mono Variable",
  primaryColor: "orange",
  defaultRadius: 0,
  components: {
    AppShell: AppShell.extend({
      defaultProps: {
        c: "white",
      },
    }),
    NavLink: NavLink.extend({
      defaultProps: {
        variant: "filled",
        autoContrast: true,
      },
    }),
    Card: Card.extend({
      defaultProps: {
        withBorder: true,
        bg: "#1c1917",
      },
    }),
    Badge: Badge.extend({
      defaultProps: {
        radius: 0,
      },
    }),
    ThemeIcon: ThemeIcon.extend({
      defaultProps: {
        variant: "light",
        size: "xl",
      },
    }),
    Tooltip: Tooltip.extend({
      defaultProps: {
        withArrow: true,
      },
    }),
    Accordion: Accordion.extend({
      defaultProps: {
        styles: {
          item: {
            backgroundColor: "#1c1917",
          },
        },
      },
    }),
    Table: Table.extend({
      defaultProps: {
        c: "white",
        bg: "#1c1917",
      },
    }),
  },
});
