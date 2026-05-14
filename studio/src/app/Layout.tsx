import { NavLink, Outlet } from 'react-router-dom';
import { useHealth } from '../api/queries';
import { useSSE } from '../api/sse';
import { StatusBar } from '../components/StatusBar';

const navItems = [
  { to: '/timeline', label: 'Timeline', icon: '⏱' },
  { to: '/scenarios', label: 'Scenarios', icon: '📋' },
  { to: '/faults', label: 'Faults', icon: '⚡' },
  { to: '/services', label: 'Services', icon: '🔌' },
];

export function Layout() {
  const health = useHealth();
  const sse = useSSE();

  return (
    <div className="layout">
      <nav className="nav-rail">
        <div className="nav-brand">LC</div>
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}
          >
            <span className="nav-icon">{item.icon}</span>
            <span className="nav-label">{item.label}</span>
          </NavLink>
        ))}
      </nav>
      <div className="main-area">
        <StatusBar health={health.data} sseStatus={sse.status} />
        <main className="content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
