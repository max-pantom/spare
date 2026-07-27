//go:build desktop && linux

#include <gtk/gtk.h>
#include <libayatana-appindicator/app-indicator.h>
#include <stdlib.h>
#include <string.h>

extern void spareLinuxTrayAction(char *action);

static AppIndicator *spare_indicator = NULL;
static GtkWidget *spare_menu = NULL;
static gboolean spare_visible = TRUE;

typedef struct {
    char *status;
    char *open_label;
    char *toggle_label;
    int has_instance;
    int running;
} SpareTrayUpdate;

static void spare_action(GtkMenuItem *item, gpointer data) {
    (void)item;
    spareLinuxTrayAction((char *)data);
}

static GtkWidget *spare_action_item(const char *label, const char *action) {
    GtkWidget *item = gtk_menu_item_new_with_label(label);
    g_signal_connect(item, "activate", G_CALLBACK(spare_action), (gpointer)action);
    return item;
}

static GtkWidget *spare_disabled_item(const char *label) {
    GtkWidget *item = gtk_menu_item_new_with_label(label);
    gtk_widget_set_sensitive(item, FALSE);
    return item;
}

static void spare_replace_menu(const SpareTrayUpdate *update) {
    if (spare_indicator == NULL) {
        return;
    }
    GtkWidget *menu = gtk_menu_new();
    gtk_menu_shell_append(GTK_MENU_SHELL(menu), spare_disabled_item("Spare"));
    gtk_menu_shell_append(GTK_MENU_SHELL(menu), spare_disabled_item(update->status));
    gtk_menu_shell_append(GTK_MENU_SHELL(menu), gtk_separator_menu_item_new());
    gtk_menu_shell_append(
        GTK_MENU_SHELL(menu),
        spare_action_item(update->open_label, update->has_instance ? "open_recipe" : "choose")
    );
    if (update->has_instance) {
        gtk_menu_shell_append(GTK_MENU_SHELL(menu), spare_action_item("Show QR", "share"));
        gtk_menu_shell_append(GTK_MENU_SHELL(menu), spare_action_item(update->toggle_label, "toggle"));
        gtk_menu_shell_append(GTK_MENU_SHELL(menu), spare_action_item("Recent activity", "activity"));
    }
    gtk_menu_shell_append(GTK_MENU_SHELL(menu), spare_action_item("Open Spare", "open_spare"));
    gtk_menu_shell_append(GTK_MENU_SHELL(menu), gtk_separator_menu_item_new());
    gtk_menu_shell_append(GTK_MENU_SHELL(menu), spare_action_item("Quit Spare", "quit"));
    gtk_widget_show_all(menu);
    app_indicator_set_menu(spare_indicator, GTK_MENU(menu));
    if (spare_menu != NULL) {
        gtk_widget_destroy(spare_menu);
    }
    spare_menu = menu;
}

static gboolean spare_start_idle(gpointer data) {
    (void)data;
    if (spare_indicator != NULL) {
        return G_SOURCE_REMOVE;
    }
    spare_indicator = app_indicator_new(
        "spare",
        "folder-publicshare-symbolic",
        APP_INDICATOR_CATEGORY_APPLICATION_STATUS
    );
    app_indicator_set_title(spare_indicator, "Spare");
    app_indicator_set_status(
        spare_indicator,
        spare_visible ? APP_INDICATOR_STATUS_ACTIVE : APP_INDICATOR_STATUS_PASSIVE
    );
    SpareTrayUpdate initial = {
        .status = "No active job",
        .open_label = "Choose a job",
        .toggle_label = "",
        .has_instance = 0,
        .running = 0,
    };
    spare_replace_menu(&initial);
    return G_SOURCE_REMOVE;
}

static gboolean spare_update_idle(gpointer data) {
    SpareTrayUpdate *update = (SpareTrayUpdate *)data;
    spare_replace_menu(update);
    free(update->status);
    free(update->open_label);
    free(update->toggle_label);
    free(update);
    return G_SOURCE_REMOVE;
}

static gboolean spare_visible_idle(gpointer data) {
    gboolean visible = GPOINTER_TO_INT(data) != 0;
    spare_visible = visible;
    if (spare_indicator != NULL) {
        app_indicator_set_status(
            spare_indicator,
            visible ? APP_INDICATOR_STATUS_ACTIVE : APP_INDICATOR_STATUS_PASSIVE
        );
    }
    return G_SOURCE_REMOVE;
}

static gboolean spare_stop_idle(gpointer data) {
    (void)data;
    if (spare_indicator != NULL) {
        app_indicator_set_status(spare_indicator, APP_INDICATOR_STATUS_PASSIVE);
        g_object_unref(spare_indicator);
        spare_indicator = NULL;
    }
    if (spare_menu != NULL) {
        gtk_widget_destroy(spare_menu);
        spare_menu = NULL;
    }
    return G_SOURCE_REMOVE;
}

void spare_linux_tray_start(void) {
    g_idle_add(spare_start_idle, NULL);
}

void spare_linux_tray_update(
    const char *status,
    const char *open_label,
    const char *toggle_label,
    int has_instance,
    int running
) {
    SpareTrayUpdate *update = malloc(sizeof(SpareTrayUpdate));
    update->status = strdup(status != NULL ? status : "No active job");
    update->open_label = strdup(open_label != NULL ? open_label : "Choose a job");
    update->toggle_label = strdup(toggle_label != NULL ? toggle_label : "");
    update->has_instance = has_instance;
    update->running = running;
    g_idle_add(spare_update_idle, update);
}

void spare_linux_tray_set_visible(int visible) {
    g_idle_add(spare_visible_idle, GINT_TO_POINTER(visible));
}

void spare_linux_tray_stop(void) {
    g_idle_add(spare_stop_idle, NULL);
}
