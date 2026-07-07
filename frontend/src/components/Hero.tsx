import { Link } from "react-router-dom";
import { Button } from "./Button";
import styles from "./Hero.module.css";

export default function Hero() {
  return (
    <section className={styles.hero}>
      <div className="container">
        <h1 className={styles.title}>
          Быстрый VLESS VPN
          <br />с <span className={styles.accent}>Reality</span>
        </h1>
        <p className={styles.subtitle}>
          Стабильные подключения, без логов, низкий пинг и защита от блокировок. Начни за 2 минуты.
        </p>
        <div className={styles.actions}>
          <Link to="/register">
            <Button>Начать</Button>
          </Link>
          <Link to="/pricing">
            <Button variant="secondary">Тарифы</Button>
          </Link>
        </div>
      </div>
    </section>
  );
}
