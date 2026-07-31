import { Link } from "react-router-dom";
import styles from "./Navbar.module.css";

export default function Navbar() {
  return (
    <nav className={styles.nav}>
      <div className={styles.inner}>
        <Link to="/" className={styles.brand}>
          <span className={styles.brandIcon}>●</span>
          Nexus
        </Link>

        <div className={styles.links}>
          <Link to="/pricing">Тарифы</Link>
          <Link to="/dashboard/instructions">Инструкции</Link>
        </div>

        <div className={styles.actions}>
          <Link to="/login">Вход</Link>
          <Link to="/register" className="button-primary">
            Регистрация
          </Link>
        </div>
      </div>
    </nav>
  );
}
