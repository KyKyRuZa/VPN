import styles from "./Footer.module.css";

export default function Footer() {
  return (
    <footer className={styles.footer}>
      <div className={styles.inner}>
        <span>© {new Date().getFullYear()} VPNify</span>
        <span>VLESS + Reality • Без логов • Высокая скорость</span>
      </div>
    </footer>
  );
}
