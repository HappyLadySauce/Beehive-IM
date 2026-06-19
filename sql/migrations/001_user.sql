CREATE TABLE IF NOT EXISTS `users` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '用户ID',
    `username` VARCHAR(50) NOT NULL UNIQUE COMMENT '用户名',
    `email` VARCHAR(255) NOT NULL UNIQUE COMMENT '邮箱',
    `phone` VARCHAR(20) NOT NULL UNIQUE COMMENT '手机号',
    `password_hash` VARCHAR(255) NOT NULL COMMENT '密码哈希',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at` DATETIME NULL COMMENT '删除时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_users_username` (`username`),
    UNIQUE KEY `uk_users_email` (`email`),
    UNIQUE KEY `uk_users_phone` (`phone`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户信息表';

CREATE TABLE IF NOT EXISTS `user_profiles` (
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    `avatar` VARCHAR(255) NULL COMMENT '头像',
    `nickname` VARCHAR(50) NULL COMMENT '昵称',
    `gender` TINYINT UNSIGNED NULL COMMENT '性别',
    `birthday` DATE NULL COMMENT '生日',
    `address` VARCHAR(255) NULL COMMENT '地址',
    `city` VARCHAR(50) NULL COMMENT '城市',
    `province` VARCHAR(50) NULL COMMENT '省份',
    `country` VARCHAR(50) NULL COMMENT '国家',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at` DATETIME NULL COMMENT '删除时间',
    PRIMARY KEY (`user_id`),
    CONSTRAINT `fk_user_profiles_user_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户详情表';
