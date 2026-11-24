// Theme toggle, mobile navigation, copy buttons and screenshot zoom. No dependencies.

(function () {
  const root = document.documentElement;
  const stored = localStorage.getItem("authapi-theme");
  if (stored === "dark" || stored === "light") {
    root.setAttribute("data-theme", stored);
  }

  const toggle = document.querySelector(".theme-toggle");
  if (toggle) {
    toggle.addEventListener("click", function () {
      const current = root.getAttribute("data-theme") ||
        (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
      const next = current === "dark" ? "light" : "dark";
      root.setAttribute("data-theme", next);
      localStorage.setItem("authapi-theme", next);
    });
  }

  const navToggle = document.querySelector(".nav-toggle");
  const sidebar = document.querySelector(".sidebar");
  if (navToggle && sidebar) {
    navToggle.addEventListener("click", function () {
      const open = sidebar.classList.toggle("open");
      navToggle.setAttribute("aria-expanded", String(open));
    });
  }

  document.querySelectorAll("pre").forEach(function (pre) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "copy-btn";
    button.textContent = "Copy";
    button.addEventListener("click", function () {
      const code = pre.querySelector("code");
      const text = code ? code.innerText : pre.innerText;
      navigator.clipboard.writeText(text).then(function () {
        button.textContent = "Copied";
        setTimeout(function () { button.textContent = "Copy"; }, 1500);
      });
    });
    pre.appendChild(button);
  });

  // Lightbox for screenshots: click (or press Enter on) an image to view it full size.
  const images = Array.from(document.querySelectorAll(".shot img, .content img"));
  if (images.length) {
    const box = document.createElement("div");
    box.className = "lightbox";
    box.setAttribute("role", "dialog");
    box.setAttribute("aria-modal", "true");
    box.setAttribute("aria-label", "Screenshot preview");
    box.innerHTML = `
      <button class="lightbox-close" type="button" aria-label="Close preview">&times;</button>
      <button class="lightbox-nav lightbox-prev" type="button" aria-label="Previous image">&lsaquo;</button>
      <figure class="lightbox-figure">
        <img alt="">
        <figcaption hidden></figcaption>
      </figure>
      <button class="lightbox-nav lightbox-next" type="button" aria-label="Next image">&rsaquo;</button>`;
    document.body.appendChild(box);

    const preview = box.querySelector("img");
    const caption = box.querySelector("figcaption");
    let index = 0;
    let restoreFocus = null;

    function fullSource(source) {
      if (source.dataset.full) return source.dataset.full;
      const src = source.currentSrc || source.src;
      return src.includes("-1440.webp") ? src.replace("-1440.webp", "-2880.webp") : src;
    }

    function open(i, trigger) {
      index = (i + images.length) % images.length;
      const source = images[index];
      preview.onerror = function () {
        // Fall back to the inline size if a 2x asset is missing.
        preview.onerror = null;
        preview.src = source.currentSrc || source.src;
      };
      preview.src = fullSource(source);
      preview.alt = source.alt || "";
      const figure = source.closest("figure");
      const figureCaption = figure ? figure.querySelector("figcaption") : null;
      const text = figureCaption ? figureCaption.textContent.trim() : (source.alt || "");
      caption.textContent = text;
      caption.hidden = !text;
      box.classList.add("open");
      document.body.classList.add("lightbox-open");
      restoreFocus = trigger || document.activeElement;
      box.querySelector(".lightbox-close").focus();
    }

    function close() {
      box.classList.remove("open");
      document.body.classList.remove("lightbox-open");
      preview.removeAttribute("src");
      if (restoreFocus && typeof restoreFocus.focus === "function") restoreFocus.focus();
    }

    images.forEach(function (source, i) {
      source.tabIndex = 0;
      source.setAttribute("role", "button");
      source.setAttribute("aria-label", "Open image: " + (source.alt || "screenshot"));
      source.addEventListener("click", function () { open(i, source); });
      source.addEventListener("keydown", function (event) {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          open(i, source);
        }
      });
    });

    box.addEventListener("click", function (event) {
      if (event.target === box || event.target.classList.contains("lightbox-close")) {
        close();
      }
    });
    box.querySelector(".lightbox-prev").addEventListener("click", function () { open(index - 1); });
    box.querySelector(".lightbox-next").addEventListener("click", function () { open(index + 1); });
    document.addEventListener("keydown", function (event) {
      if (!box.classList.contains("open")) return;
      if (event.key === "Escape") close();
      if (event.key === "ArrowLeft") open(index - 1);
      if (event.key === "ArrowRight") open(index + 1);
    });
  }

  // Highlight the current section in the table of contents.
  const tocLinks = Array.from(document.querySelectorAll(".toc a"));
  if (tocLinks.length && "IntersectionObserver" in window) {
    const observer = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (!entry.isIntersecting) return;
        tocLinks.forEach(function (link) {
          link.style.color = link.getAttribute("href") === "#" + entry.target.id ? "var(--accent)" : "";
        });
      });
    }, { rootMargin: "-80px 0px -70% 0px" });
    tocLinks.forEach(function (link) {
      const target = document.querySelector(link.getAttribute("href"));
      if (target) observer.observe(target);
    });
  }
})();
