gsap.registerPlugin(SplitText,ScrollTrigger,CustomEase);

const disableLenis = document.body.hasAttribute('data-no-lenis');

let lenis = null;

if (!disableLenis) {
  lenis = new Lenis({
    smooth: true,
    lerp: 0.3,
    direction: "vertical",
  });

  function raf(time) {
    lenis.raf(time);
    requestAnimationFrame(raf);
  }

  requestAnimationFrame(raf);
}

let animationComplete = false;

function initSmoothScroll() {
  const underlineLinks = document.querySelectorAll(
    'a.underline-link[href^="#"]'
  );
  underlineLinks.forEach((link) => {
    link.addEventListener("click", function (e) {
      e.preventDefault();
      e.stopPropagation();
      const targetId = this.getAttribute("href");
      if (targetId && targetId !== "#") {
        const targetElement = document.querySelector(targetId);
        if (targetElement) {
          if (history.pushState) {
            history.pushState(null, null, targetId);
          }
          let scrollOffset;
          if (animationComplete) {
            scrollOffset = -170;
            console.log(animationComplete);
          } else {
            scrollOffset = -230;
            console.log(animationComplete);
          }
          console.log(scrollOffset);
          if (lenis) {
            lenis.scrollTo(targetElement, {
              offset: scrollOffset,
              duration: 0.5,
              lock: true,
              immediate: false,
            });
          } else {
            const offsetTop = targetElement.getBoundingClientRect().top + window.scrollY + scrollOffset;
            window.scrollTo({ top: offsetTop, behavior: 'smooth' });
          }
        }
      }
      return false;
    });
  });
}

document.addEventListener("DOMContentLoaded", initSmoothScroll);


document.querySelectorAll("#navmenu-button").forEach((element) => {
  element.addEventListener("click", function () {
    element.classList.toggle("stop-scroll");
    if (typeof lenis !== "undefined" && lenis) {
      if (element.classList.contains("stop-scroll")) {
        lenis.stop();
      } else {
        lenis.start();
      }
    }
  });
});

//cover animation
document.addEventListener("DOMContentLoaded", function () {
  gsap.set("[data-page-heading]", { opacity: 0 });
  gsap.set("#cover-description-animation", { opacity: 0, y: 60 });
  gsap.set(".cover-buttons-box", { opacity: 0, y: 60 });
  gsap.set(".cover-render-box", { opacity: 0, y: 60 });
  
  const heading = document.querySelector("[data-page-heading]");
  if (heading) {
    heading.style.visibility = 'hidden';
  }

  function initAnimations() {
    if (!heading) return;
    
    heading.style.visibility = 'visible';
    
    requestAnimationFrame(() => {
      const split = new SplitText(heading, {
        type: "words",
        wordsClass: "word",
      });

      const tl = gsap.timeline();
      
      tl.set("[data-page-heading]", { opacity: 1 });
      
      tl.fromTo(
        split.words,
        {
          opacity: 0,
          y: 60,
        },
        {
          opacity: 1,
          y: 0,
          duration: 0.5,
          ease: "power2.out",
          stagger: {
            amount: 0.3, 
            from: "start"
          },
        },
        0 
      );

      const wordsDuration = Math.min(0.5 + (split.words.length * 0.05), 1.5);
      
      tl.to(
        "#cover-description-animation",
        {
          opacity: 1,
          y: 0,
          duration: 0.5,
          ease: "power2.out",
        },
        wordsDuration * 0.7 
      );

      tl.to(
        [".cover-buttons-box", ".cover-render-box"],
        {
          opacity: 1,
          y: 0,
          duration: 0.5,
          ease: "power2.out",
          stagger: 0.05,
        },
        wordsDuration * 0.8
      );
    });
  }


  if (document.fonts && document.fonts.ready) {
    document.fonts.ready.then(() => {
      setTimeout(initAnimations, 100);
    });
  } else {
    setTimeout(initAnimations, 300);
  }
});
//modes animation
function initModesAnimations() {
  const animatedElements = document.querySelectorAll("[data-modes-animation]");

  animatedElements.forEach((element) => {
    element.setAttribute("data-animation-completed", "false");

    gsap.set(element, {
      opacity: 0,
      y: 60,
    });

    gsap.to(element, {
      opacity: 1,
      y: 0,
      duration: 0.5,
      ease: "cubic-bezier(0.25, 0.8, 0.25, 1)",

      scrollTrigger: {
        trigger: element,
        start: "top 90%",
        end: "bottom 20%",
        toggleActions: "play none none none",
        onComplete: () => {
          animationComplete = true;
        },
      },
    });
  });
}

document.addEventListener("DOMContentLoaded", initModesAnimations);

//basic animation
function initBasicAnimations() {
  const animatedElements = document.querySelectorAll("[data-basic-animation]");

  animatedElements.forEach((element) => {
    gsap.set(element, {
      opacity: 0,
      y: 60,
    });

    gsap.to(element, {
      opacity: 1,
      y: 0,
      duration: 0.5,
      ease: "cubic-bezier(0.25, 0.8, 0.25, 1)",

      scrollTrigger: {
        trigger: element,
        start: "top 90%",
        end: "bottom 20%",
        toggleActions: "play none none none",
      },
    });
  });
}

document.addEventListener("DOMContentLoaded", initBasicAnimations);

//basic fade animation
function initBasicFadeAnimations() {
  const animatedElements = document.querySelectorAll("[data-fade-animation]");

  animatedElements.forEach((element) => {
    gsap.set(element, {
      opacity: 0,
    });

    gsap.to(element, {
      opacity: 1,
      duration: 0.7,
      ease: "cubic-bezier(0.25, 0.8, 0.25, 1)",

      scrollTrigger: {
        trigger: element,
        start: "top 90%",
        end: "bottom 20%",
        toggleActions: "play none none none",
      },
    });
  });
}

document.addEventListener("DOMContentLoaded", function () {
  document.fonts.ready.then(() => {
    const animatedHeadings = document.querySelectorAll("[data-heading-animation]");

    animatedHeadings.forEach((heading) => {
      heading.style.visibility = "hidden";

      requestAnimationFrame(() => {
        heading.style.visibility = "visible";

        const split = new SplitText(heading, {
          type: "words",
          wordsClass: "word",
        });

        gsap.set(heading, { opacity: 1 });

        gsap.fromTo(
          split.words,
          {
            opacity: 0,
            y: 30,
          },
          {
            opacity: 1,
            y: 0,
            duration: 0.4,
            ease: "power2.out",
            stagger: {
              amount: 0.2,
              from: "start",
            },
            scrollTrigger: {
              trigger: heading,
              start: "top 90%",
              toggleActions: "play none none none",
            },
          }
        );
      });
    });
  });
});

document.addEventListener('DOMContentLoaded', function() {
  
  const secondaryMenu = document.querySelector('.mobile-secondary-menu');
  const dropdownToggle = document.querySelector('.navlink-dropdown');
  const menuButton = document.querySelector('.menu-button');
  
  if (!secondaryMenu) return;
  
  if (dropdownToggle) {
    dropdownToggle.addEventListener('click', function() {
      if (window.innerWidth > 480) return;
      setTimeout(function() {
        secondaryMenu.style.transform = 'translateX(0%)';
      }, 10);
    });
  }
  
  document.addEventListener('click', function(e) {
    if (window.innerWidth > 480) return;
    
    const back = e.target.closest('.dropdown-mobile-icon');
    if (back) {
      e.preventDefault();
      e.stopPropagation();
      e.stopImmediatePropagation();
      secondaryMenu.style.transform = 'translateX(100%)';
      return;
    }
    
    const close = e.target.closest('.dropdown-mobile-close-icon');
    if (close) {
      e.preventDefault();
      e.stopPropagation();
      e.stopImmediatePropagation();
      secondaryMenu.style.transform = 'translateX(100%)';
      if (menuButton) menuButton.click();
      return;
    }
    
    if (e.target.closest('.mobile-secondary-menu')) {
      const link = e.target.closest('a');
      if (link) return;
      
      e.preventDefault();
      e.stopPropagation();
      e.stopImmediatePropagation();
    }
  }, true);
  
});

//product pages animation
  document.addEventListener("DOMContentLoaded", function () {
  const coverBox = document.querySelector(".template-cover-box");
  if (!coverBox) return;

  const heading = coverBox.querySelector("h1");
  const description = coverBox.querySelector(".text-size-18");
  const button = coverBox.querySelector(".button");
  const image = coverBox.querySelector(".template-cover-image");

  gsap.set(heading, { opacity: 0 });
  gsap.set(description, { opacity: 0, y: 60 });
  gsap.set(button, { opacity: 0, y: 60 });
  gsap.set(image, { opacity: 0, y: 60 });

  if (heading) {
    heading.style.visibility = "hidden";
  }

  function initAnimations() {
    if (!heading) return;

    heading.style.visibility = "visible";

    requestAnimationFrame(() => {
      const split = new SplitText(heading, {
        type: "words",
        wordsClass: "word",
      });

      const tl = gsap.timeline();

      tl.set(heading, { opacity: 1 });

      tl.fromTo(
        split.words,
        {
          opacity: 0,
          y: 60,
        },
        {
          opacity: 1,
          y: 0,
          duration: 0.5,
          ease: "power2.out",
          stagger: {
            amount: 0.3,
            from: "start",
          },
        },
        0
      );

      const wordsDuration = Math.min(0.5 + split.words.length * 0.05, 1.5);

      tl.to(
        description,
        {
          opacity: 1,
          y: 0,
          duration: 0.5,
          ease: "power2.out",
        },
        wordsDuration * 0.7
      );

      tl.to(
        button,
        {
          opacity: 1,
          y: 0,
          duration: 0.5,
          ease: "power2.out",
        },
        wordsDuration * 0.8
      );

      tl.to(
        image,
        {
          opacity: 1,
          y: 0,
          duration: 0.5,
          ease: "power2.out",
        },
        wordsDuration * 0.9
      );
    });
  }

  if (document.fonts && document.fonts.ready) {
    document.fonts.ready.then(() => {
      setTimeout(initAnimations, 100);
    });
  } else {
    setTimeout(initAnimations, 300);
  }
});

var mySwiper;

function initSwiper() {
    mySwiper = new Swiper(".how-it-work-swiper", {
      autoplay: {
        delay: 9000, 
        },
      navigation: {
        nextEl: ".how-it-work-next-arrow",
        prevEl: ".how-it-work-previous-arrow",
      },
      pagination: {
      el: '.how-it-work-swiper-pagination',
        clickable: 'true',
        type: 'bullets',
        renderBullet: function (index, className) {
          return '<span class="' + className + '">' + '<b></b>' + '</span>';
        },
      },
    });
    
    mySwiper.autoplay.stop();
}

initSwiper();

const observer = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        if (entry.isIntersecting) {
            mySwiper.autoplay.start();
        } else {
            mySwiper.autoplay.stop();
        }
    });
}, {
    threshold: 0.1 
});

const swiperContainer = document.querySelector('.how-it-work-swiper');
if (swiperContainer) {
    observer.observe(swiperContainer);
}

function animateElements(selector) {
  const elements = document.querySelectorAll(selector);
  if (!elements.length) return;
  
  gsap.set(elements, {
    opacity: 0,
    y: 60
  });
  
  gsap.to(elements, {
    opacity: 1,
    y: 0,
    duration: 0.5,
    ease: 'cubic-bezier(0.25, 0.8, 0.25, 1)',
    stagger: 0.05,
    scrollTrigger: {
      trigger: elements[0],
        start: 'top 90%',
        end: 'bottom 20%',
      toggleActions: 'play none none none'
    }
  });
}

function initAllAnimations() {
  animateElements('.features-card');
  animateElements('.investor-item');
 
}

document.addEventListener('DOMContentLoaded', initAllAnimations);