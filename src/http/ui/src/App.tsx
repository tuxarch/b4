import { NavLink as RouterNavLink, useLocation } from "react-router";

import {
  AppShell,
  Box,
  Burger,
  Group,
  MantineProvider,
  NavLink,
  Title,
} from "@mantine/core";
import "@mantine/core/styles.css";
import { useDisclosure } from "@mantine/hooks";
import { Notifications } from "@mantine/notifications";
import "@mantine/notifications/styles.css";

interface NavItem {
  path: string;
  label: string;
  icon: React.ReactNode;
}

const navItems: NavItem[] = [
  { path: "/dashboard", label: "Dashboard", icon: <></> },
  { path: "/sets", label: "Sets", icon: <></> },
  { path: "/discovery", label: "Discovery", icon: <></> },
  { path: "/connections", label: "Connections", icon: <></> },
  { path: "/logs", label: "Logs", icon: <></> },
  { path: "/settings", label: "Settings", icon: <></> },
];

export default function App() {
  const location = useLocation();
  const [mobileOpened, { toggle: toggleMobile }] = useDisclosure();
  const [desktopOpened, { toggle: toggleDesktop }] = useDisclosure(true);

  // dolboeb ili geniy?
  const title = navItems.find((item) => item.path === location.pathname)?.label;

  return (
    <MantineProvider defaultColorScheme="dark">
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
          <AppShell.Section>{/*<Version />*/}</AppShell.Section>
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
