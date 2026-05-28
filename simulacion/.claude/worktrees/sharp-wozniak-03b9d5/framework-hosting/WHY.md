# WHY - Framework Hosting

Hosting existe porque configurar SMTP, DNS y cuentas de email en un hosting
requiere acceso a cPanel y conocimiento técnico que el usuario no tiene por
qué tener.

Hosting conecta al cPanel del usuario y provisiona la infraestructura que
Mensajero necesita para enviar.

## Problema Que Resuelve

Sin Hosting, configurar una cuenta de email para enviar cobros implica entrar
al cPanel, navegar menús, crear la cuenta, copiar credenciales SMTP y pegarlas
en algún lado. Hosting hace eso con un comando.

## Relación Con Otros Frameworks

- **Mensajero** consume las credenciales SMTP que Hosting provisiona.
- **Sabio** puede guardar los emails del dominio provisionado.
- **Deployer** puede necesitar Hosting para configurar DNS de servicios.

Hosting no envía emails. Mensajero no configura SMTP.
