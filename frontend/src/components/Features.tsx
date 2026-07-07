import styles from "./Features.module.css";

const items = [
  {
    title: "VLESS + Reality",
    desc: "Обход DPI и блокировок с маскировкой под обычный HTTPS-трафик.",
  },
  { title: "Низкий пинг", desc: "Оптимизированные маршруты и быстрые серверы в EU/Asia." },
  { title: "Без логов", desc: "Мы не собираем и не храним данные о вашей активности." },
];

export default function Features() {
  return (
    <section className="section">
      <div className="container">
        <div className={styles.grid}>
          {items.map((it) => (
            <div key={it.title} className={styles.item}>
              <div className={styles.icon}>◆</div>
              <div className={styles.title}>{it.title}</div>
              <div className={styles.desc}>{it.desc}</div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
