import { NavLink as RouterNavLink, useLocation } from "react-router";

import {
  IconConnections,
  IconDashboard,
  IconDiscovery,
  IconLogs,
  IconSets,
  IconSettings,
} from "@b4.icons";
import Version from "@components/version/Version";
import {
  AppShell,
  Box,
  Burger,
  Divider,
  Group,
  MantineProvider,
  NavLink,
  Title,
} from "@mantine/core";
import "@mantine/core/styles.css";
import { useDisclosure } from "@mantine/hooks";
import { Notifications } from "@mantine/notifications";
import "@mantine/notifications/styles.css";
import { theme } from "./design/theme";
import "./design/yanenavizhumantine.css";

interface NavItem {
  path: string;
  label: string;
  icon: React.ReactNode;
}

const navItems: NavItem[] = [
  { path: "/dashboard", label: "Dashboard", icon: <IconDashboard /> },
  { path: "/sets", label: "Sets", icon: <IconSets /> },
  { path: "/discovery", label: "Discovery", icon: <IconDiscovery /> },
  { path: "/connections", label: "Connections", icon: <IconConnections /> },
  { path: "/logs", label: "Logs", icon: <IconLogs /> },
  { path: "/settings", label: "Settings", icon: <IconSettings /> },
];

export default function App() {
  const location = useLocation();
  const [mobileOpened, { toggle: toggleMobile }] = useDisclosure();
  const [desktopOpened, { toggle: toggleDesktop }] = useDisclosure(true);

  // dolboeb ili geniy?
  const title = navItems.find((item) => item.path === location.pathname)?.label;

  return (
    <MantineProvider defaultColorScheme="dark" theme={theme}>
      <Notifications />
      <AppShell
        layout="alt"
        header={{ height: { base: 60, lg: 70 } }}
        navbar={{
          width: { base: 200, lg: 300 },
          breakpoint: "sm",
          collapsed: { mobile: !mobileOpened, desktop: !desktopOpened },
        }}
      >
        <AppShell.Header>
          <Group h="100%" px="md">
            <Burger
              opened={mobileOpened}
              onClick={toggleMobile}
              hiddenFrom="sm"
              size="sm"
            />
            <Burger
              opened={desktopOpened}
              onClick={toggleDesktop}
              visibleFrom="sm"
              size="sm"
            />
            <Title>{title}</Title>
          </Group>
        </AppShell.Header>

        <AppShell.Navbar p="md">
          <AppShell.Section p="md">{/*<Logo />*/}</AppShell.Section>
          <Divider />
          <AppShell.Section grow p="md">
            <Box>
              {navItems.map((item) => (
                <NavLink
                  key={item.path}
                  component={RouterNavLink}
                  to={item.path}
                  label={item.label}
                  leftSection={item.icon}
                />
              ))}
            </Box>
          </AppShell.Section>
          <Divider />
          <AppShell.Section p="md">
            <Version />
          </AppShell.Section>
        </AppShell.Navbar>
      </AppShell>
      {/*
          <Routes>
            <Route path="/" element={<Navigate to="/dashboard" replace />} />
            <Route path="/dashboard" element={<DashboardPage />} />
            <Route path="/sets/*" element={<SetsPage />} />
            <Route path="/connections" element={<ConnectionsPage />} />
            <Route path="/discovery" element={<DiscoveryPage />} />
            <Route path="/logs" element={<LogsPage />} />
            <Route path="/settings/*" element={<SettingsPage />} />
            <Route path="*" element={<Navigate to="/dashboard" replace />} />
          </Routes>

      */}
    </MantineProvider>
  );
}
